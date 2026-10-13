package wire

import (
	"encoding/binary"
	"slices"
	"strings"

	"golang.org/x/net/http2/hpack"
)

func (c *http2Conn) sendHeaders() error {
	fields, err := c.decoder.DecodeFull(c.headerBlock)
	if err != nil {
		return err
	}
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
	order := append(slices.Clone(c.config.PseudoHeaderOrder), explicit...)
	order = append(order, c.config.HeaderOrder(fetchMode)...)
	slices.SortStableFunc(fields, func(a, b hpack.HeaderField) int {
		return compareHeaderNames(order, a.Name, b.Name)
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
		var payload []byte
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
				payload = binary.BigEndian.AppendUint32(nil, dep)
				payload = append(payload, c.config.Priority.Weight)
			}
		}
		n := min(size, c.encoded.Len())
		payload = append(payload, c.encoded.Next(n)...)
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
