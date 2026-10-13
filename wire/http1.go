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
}

// Begin marks the next native HTTP/1 request header block.
func (c *HTTP1Conn) Begin(order []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.headers = true
	c.pending = nil
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
	lines := strings.Split(string(c.pending[:end]), "\r\n")
	var explicit []string
	kept := lines[:1]
	for _, line := range lines[1:] {
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(name, headerOrderKey) {
			explicit = append(explicit, headerOrder(value)...)
			continue
		}
		if ok && strings.HasPrefix(strings.ToLower(name), "sec-ch-") {
			line = strings.ToLower(name) + ":" + value
		}
		kept = append(kept, line)
	}
	lines = kept
	order := append(explicit, c.order...)
	slices.SortStableFunc(lines[1:], func(a, b string) int {
		a, _, _ = strings.Cut(a, ":")
		b, _, _ = strings.Cut(b, ":")

		return compareHeaderNames(order, strings.ToLower(a), strings.ToLower(b))
	})
	data := []byte(strings.Join(lines, "\r\n") + "\r\n\r\n")
	data = append(data, c.pending[end+4:]...)
	c.pending = nil
	c.headers = false
	if n, err := bytes.NewReader(data).WriteTo(c.Conn); err != nil {
		// Preserve a partial write: reporting zero could make net/http retry a POST.
		return min(int(n), len(p)), err
	}

	return len(p), nil
}
