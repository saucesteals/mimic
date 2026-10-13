package wire

import (
	"bytes"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
)

// HTTP1Conn changes only the header block emitted by the standard library. Begin
// is called by the native GotConn trace before request serialization.
type HTTP1Conn struct {
	net.Conn
	mu      sync.Mutex
	headers bool
	pending []byte
	order   []string
	fields  []http1Header
	output  []byte
}

type http1Header struct {
	line string
	name string
	rank int
}

// Begin marks the next native HTTP/1 request header block.
func (c *HTTP1Conn) Begin(order []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers = true
	c.pending = c.pending[:0]
	c.order = order
}

func (c *HTTP1Conn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.headers {
		return c.Conn.Write(p)
	}
	c.pending = append(c.pending, p...)
	end := bytes.Index(c.pending, []byte("\r\n\r\n"))
	if end < 0 {
		if len(c.pending) > 1<<20 {
			return 0, fmt.Errorf("header block too large")
		}
		return len(p), nil
	}
	defer func() {
		clear(c.fields)
		c.fields = c.fields[:0]
		c.pending = c.pending[:0]
		c.output = c.output[:0]
		// An unusually large header block should not enlarge the idle connection.
		if cap(c.fields) > 256 {
			c.fields = nil
		}
		if cap(c.pending) > 64<<10 {
			c.pending = nil
		}
		if cap(c.output) > 64<<10 {
			c.output = nil
		}
	}()
	first, rest, _ := strings.Cut(string(c.pending[:end]), "\r\n")
	var explicit []string
	for rest != "" {
		line, tail, _ := strings.Cut(rest, "\r\n")
		rest = tail
		name, value, ok := strings.Cut(line, ":")
		name = strings.ToLower(name)
		if ok && strings.EqualFold(name, headerOrderKey) {
			explicit = append(explicit, headerOrder(value)...)
			continue
		}
		if ok && strings.HasPrefix(name, "sec-ch-") {
			line = name + ":" + value
		}
		c.fields = append(c.fields, http1Header{line: line, name: name})
	}
	order := append(explicit, c.order...)
	for i := range c.fields {
		rank := slices.Index(order, c.fields[i].name)
		if rank < 0 {
			rank = len(order)
		}
		c.fields[i].rank = rank
	}
	slices.SortStableFunc(c.fields, func(a, b http1Header) int {
		if a.rank != b.rank {
			return a.rank - b.rank
		}

		return strings.Compare(a.name, b.name)
	})
	c.output = append(c.output[:0], first...)
	for _, field := range c.fields {
		c.output = append(c.output, '\r', '\n')
		c.output = append(c.output, field.line...)
	}
	c.output = append(c.output, "\r\n\r\n"...)
	c.output = append(c.output, c.pending[end+4:]...)
	c.headers = false
	if n, err := bytes.NewReader(c.output).WriteTo(c.Conn); err != nil {
		// Preserve a partial write: reporting zero could make net/http retry a POST.
		return min(int(n), len(p)), err
	}

	return len(p), nil
}
