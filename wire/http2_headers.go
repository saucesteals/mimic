package wire

import (
	"encoding/binary"
	"slices"
	"strings"

	"golang.org/x/net/http2/hpack"
)

func (c *http2Conn) sendHeaders() error {
	defer func() {
		// Reuse ordinary header blocks without retaining old header strings.
		clear(c.fields)
		clear(c.order)
		c.fields = c.fields[:0]
		c.order = c.order[:0]
		if cap(c.fields) > 256 {
			c.fields = nil
		}
		if cap(c.order) > 256 {
			c.order = nil
		}
	}()
	if _, err := c.decoder.Write(c.headerBlock); err != nil {
		return err
	}
	if err := c.decoder.Close(); err != nil {
		return err
	}
	fields := c.fields
	fetchMode := ""
	var explicit []string
	kept := fields[:0]
	for _, field := range fields {
		if strings.EqualFold(field.Name, headerOrderKey) {
			explicit = append(explicit, headerOrder(field.Value)...)
			continue
		}
		if field.Name == "sec-fetch-mode" {
			fetchMode = field.Value
		}
		kept = append(kept, field)
	}
	fields = kept
	c.order = append(c.order[:0], c.config.PseudoHeaderOrder...)
	c.order = append(c.order, explicit...)
	c.order = append(c.order, c.config.HeaderOrder(fetchMode)...)
	slices.SortStableFunc(fields, func(a, b hpack.HeaderField) int {
		return compareHeaderNames(c.order, a.Name, b.Name)
	})
	c.tableMu.Lock()
	c.encoded.Reset()
	for _, field := range fields {
		// The encoder writes only to bytes.Buffer, whose Write cannot fail.
		_ = c.encoder.WriteField(field)
	}
	c.tableMu.Unlock()

	first := true
	for first || c.encoded.Len() > 0 {
		kind, flags, size := byte(9), byte(0), 16384
		payload := c.payload[:0]
		if first {
			kind = 1
			flags = c.headerFlags
			if len(fields) > 0 && strings.HasPrefix(fields[0].Name, ":") && c.config.Priority != nil {
				flags |= 32
				size -= 5
				dep := c.config.Priority.StreamDep
				if c.config.Priority.Exclusive {
					dep |= 1 << 31
				}
				payload = binary.BigEndian.AppendUint32(payload, dep)
				payload = append(payload, c.config.Priority.Weight)
			}
		}
		n := min(size, c.encoded.Len())
		payload = append(payload, c.encoded.Next(n)...)
		c.payload = payload
		if c.encoded.Len() == 0 {
			flags |= 4
		}
		if err := c.writeFrame(kind, flags, c.headerStream, payload); err != nil {
			return err
		}
		first = false
	}

	if c.config.StreamWindowIncrement != 0 && slices.ContainsFunc(fields, func(f hpack.HeaderField) bool {
		return f.Name == ":method"
	}) {
		return c.writeFrame(8, 0, c.headerStream, binary.BigEndian.AppendUint32(nil, c.config.StreamWindowIncrement))
	}
	return nil
}
