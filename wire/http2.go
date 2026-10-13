package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"slices"
	"sync"

	"golang.org/x/net/http2/hpack"
)

type http2Conn struct {
	net.Conn
	mu                    sync.Mutex
	out                   bytes.Buffer
	incoming, headerBlock []byte
	inRemaining           int
	preface, settings     bool
	headerStream          uint32
	headerFlags           byte
	decoder               *hpack.Decoder
	encoder               *hpack.Encoder
	encoded               bytes.Buffer
	fields                []hpack.HeaderField
	order                 []string
	payload, frame        []byte
	tableMu               sync.Mutex
	config                HTTP2Config
	writeErr              error
	peerSettings          []byte
	settingsACKs          int
	ready                 chan struct{}
	readyOnce             sync.Once
}

// HTTP2Config contains only the serialization options owned by the wire adapter.
// Configure it before use; its slices must not be mutated afterward.
type HTTP2Config struct {
	Settings          []Setting
	PseudoHeaderOrder []string
	Priority          *Priority
	HeaderOrder       func(fetchMode string) []string
	EncoderLimit      int
	// PeerSettings comes from ParseHTTP2Settings; nil means no ALPS SETTINGS.
	// Call Begin before sending requests to await native application.
	PeerSettings []Setting
	// StreamWindowIncrement follows initial request HEADERS, never trailers.
	StreamWindowIncrement uint32
}

// Setting is an HTTP/2 setting identifier and value in wire order.
type Setting struct {
	ID  uint16
	Val uint32
}

// NewHTTP2 wraps a connection with browser-specific HTTP/2 serialization.
func NewHTTP2(conn net.Conn, config HTTP2Config) net.Conn {
	c := &http2Conn{
		Conn:   conn,
		config: config,
	}
	c.decoder = hpack.NewDecoder(min(uint32(config.EncoderLimit), 4096), func(field hpack.HeaderField) {
		c.fields = append(c.fields, field)
	})
	c.decoder.SetAllowedMaxDynamicTableSize(uint32(config.EncoderLimit))
	c.encoder = hpack.NewEncoder(&c.encoded)
	c.encoder.SetMaxDynamicTableSizeLimit(uint32(config.EncoderLimit))

	if config.PeerSettings != nil {
		c.ready = make(chan struct{})
		settings := config.PeerSettings
		for {
			n := min(len(settings), 16380/6)
			frame := make([]byte, 9, 9+6*n)
			frame[0], frame[1], frame[2], frame[3] = 0, byte(6*n>>8), byte(6*n), 4
			for _, setting := range settings[:n] {
				frame = binary.BigEndian.AppendUint16(frame, setting.ID)
				frame = binary.BigEndian.AppendUint32(frame, setting.Val)
			}
			c.peerSettings = append(c.peerSettings, frame...)
			c.settingsACKs++
			settings = settings[n:]
			if len(settings) == 0 {
				break
			}
		}
	}

	return c
}

// Begin waits until net/http has applied ALPS, before it opens the first stream.
func (c *http2Conn) Begin(_ []string) {
	if c.ready != nil {
		<-c.ready
	}
}

func (c *http2Conn) settingsReady() {
	if c.ready != nil {
		c.readyOnce.Do(func() { close(c.ready) })
	}
}

func (c *http2Conn) Close() error {
	c.settingsReady()

	return c.Conn.Close()
}

func (c *http2Conn) Read(p []byte) (int, error) {
	var n int
	var err error
	if len(c.peerSettings) > 0 {
		n = copy(p, c.peerSettings)
		c.peerSettings = c.peerSettings[n:]
	} else {
		n, err = c.Conn.Read(p)
	}
	if err != nil {
		c.settingsReady()
	}
	for data := p[:n]; len(data) > 0; {
		if len(c.incoming) < 9 {
			take := min(9-len(c.incoming), len(data))
			c.incoming = append(c.incoming, data[:take]...)
			data = data[take:]
			if len(c.incoming) < 9 {
				break
			}
			c.inRemaining = int(c.incoming[0])<<16 | int(c.incoming[1])<<8 | int(c.incoming[2])
		}
		kind, flags := c.incoming[3], c.incoming[4]
		settings := kind == 4 && flags&1 == 0
		take := min(c.inRemaining, len(data))
		if settings {
			c.incoming = append(c.incoming, data[:take]...)
		}
		data = data[take:]
		c.inRemaining -= take
		if c.inRemaining != 0 {
			continue
		}
		// net/http validates incoming frames; inspect only encoder settings.
		// Other payloads pass through without being copied into the adapter.
		if settings && (len(c.incoming)-9)%6 == 0 && binary.BigEndian.Uint32(c.incoming[5:9])&0x7fffffff == 0 {
			c.tableMu.Lock()
			for data := c.incoming[9:]; len(data) >= 6; data = data[6:] {
				if binary.BigEndian.Uint16(data) == 1 {
					c.encoder.SetMaxDynamicTableSize(binary.BigEndian.Uint32(data[2:]))
				}
			}
			c.tableMu.Unlock()
		}
		c.incoming = c.incoming[:0]
	}

	return n, err
}

func (c *http2Conn) Write(p []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	defer func() {
		if err != nil {
			c.writeErr = err
		}
	}()
	c.out.Write(p)
	if !c.preface {
		if c.out.Len() < 24 {
			return len(p), nil
		}
		if string(c.out.Bytes()[:24]) != "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n" {
			return 0, fmt.Errorf("bad preface")
		}
		if err := c.write(c.out.Next(24)); err != nil {
			return 0, err
		}
		c.preface = true
	}
	for c.out.Len() >= 9 {
		raw := c.out.Bytes()
		length := int(raw[0])<<16 | int(raw[1])<<8 | int(raw[2])
		if len(raw) < length+9 {
			break
		}
		size := length + 9
		raw = raw[:size]
		kind, flags, stream := raw[3], raw[4], binary.BigEndian.Uint32(raw[5:9])&0x7fffffff
		switch {
		case kind == 4 && flags&1 != 0 && c.settingsACKs > 0:
			// ALPS settings are already acknowledged by TLS. Consume only the
			// ACKs for the injected frames; subsequent wire ACKs pass through.
			c.settingsACKs--
			if c.settingsACKs == 0 {
				c.settingsReady()
			}
		case kind == 4 && flags&1 == 0 && !c.settings:
			settings := make([]byte, 6*len(c.config.Settings))
			for i, setting := range c.config.Settings {
				binary.BigEndian.PutUint16(settings[i*6:], setting.ID)
				binary.BigEndian.PutUint32(settings[i*6+2:], setting.Val)
			}
			if err := c.writeFrame(4, 0, 0, settings); err != nil {
				return 0, err
			}
			c.settings = true
		case kind == 1 || kind == 9:
			// Native request headers have no padding or priority prefix.
			if kind == 1 && flags&(8|32) != 0 {
				return 0, fmt.Errorf("unsupported native header frame layout")
			}
			if kind == 1 {
				c.headerStream = stream
				c.headerFlags = flags & 1
			}
			data := raw[9:]
			if len(c.headerBlock)+len(data) > 16<<20 {
				return 0, fmt.Errorf("encoded request headers exceed limit")
			}
			c.headerBlock = append(c.headerBlock, data...)
			if flags&4 != 0 {
				if err := c.sendHeaders(); err != nil {
					return 0, err
				}
				c.headerBlock = c.headerBlock[:0]
				c.headerStream = 0
			}
		default:
			// Preserve native batches of flow-control updates in one TLS write.
			// Splitting each frame adds a TLS record and socket write per update.
			if kind == 8 && length == 4 {
				pending := c.out.Bytes()
				for size+13 <= len(pending) && bytes.Equal(pending[size:size+4], []byte{0, 0, 4, 8}) {
					size += 13
				}
				raw = pending[:size]
			}
			if err := c.write(raw); err != nil {
				return 0, err
			}
		}
		c.out.Next(size)
	}

	return len(p), nil
}

// Priority is the optional HTTP/2 HEADERS priority tuple.
type Priority struct {
	StreamDep uint32
	Exclusive bool
	Weight    uint8
}

func (c *http2Conn) writeFrame(kind, flags byte, stream uint32, payload []byte) error {
	c.frame = append(c.frame[:0], make([]byte, 9)...)
	c.frame = append(c.frame, payload...)
	frame := c.frame
	n := len(payload)
	frame[0], frame[1], frame[2], frame[3], frame[4] = byte(n>>16), byte(n>>8), byte(n), kind, flags
	binary.BigEndian.PutUint32(frame[5:9], stream)

	return c.write(frame)
}

func (c *http2Conn) write(p []byte) error {
	_, err := bytes.NewReader(p).WriteTo(c.Conn)

	return err
}

// ParseHTTP2Settings validates ALPS and returns effective SETTINGS in ID order.
// Repeated identifiers use the last value, before any application data is sent.
// Nil means no SETTINGS frame; a non-nil empty result means an empty frame.
// ACCEPT_CH is validated but does not synthesize request headers. Unknown
// extension frames are ignored, as in HTTP/2; ordinary non-SETTINGS frames fail.
func ParseHTTP2Settings(data []byte) ([]Setting, error) {
	var settings []Setting
	positions := make(map[uint16]int)
	for len(data) > 0 {
		if len(data) < 9 {
			return nil, fmt.Errorf("wire: truncated ALPS frame")
		}
		n := int(data[0])<<16 | int(data[1])<<8 | int(data[2])
		kind, flags, stream := data[3], data[4], binary.BigEndian.Uint32(data[5:9])&0x7fffffff
		if n > len(data)-9 {
			return nil, fmt.Errorf("wire: truncated ALPS payload")
		}
		body := data[9 : 9+n]
		data = data[9+n:]
		switch kind {
		case 4:
			if stream != 0 || flags&1 != 0 || len(body)%6 != 0 {
				return nil, fmt.Errorf("wire: invalid ALPS SETTINGS")
			}
			if settings == nil {
				settings = []Setting{}
			}
			for len(body) > 0 {
				id, value := binary.BigEndian.Uint16(body), binary.BigEndian.Uint32(body[2:])
				if ((id == 2 || id == 8 || id == 9) && value > 1) ||
					(id == 4 && value > 1<<31-1) ||
					(id == 5 && (value < 16384 || value > 1<<24-1)) {
					return nil, fmt.Errorf("wire: invalid ALPS setting %d", id)
				}
				if i, ok := positions[id]; ok {
					settings[i].Val = value
				} else {
					positions[id] = len(settings)
					settings = append(settings, Setting{ID: id, Val: value})
				}
				body = body[6:]
			}
		case 0x89: // ACCEPT_CH: pairs of length-prefixed origin and field value.
			if stream != 0 || flags != 0 {
				return nil, fmt.Errorf("wire: invalid ALPS ACCEPT_CH")
			}
			for len(body) > 0 {
				for range 2 {
					if len(body) < 2 || int(binary.BigEndian.Uint16(body)) > len(body)-2 {
						return nil, fmt.Errorf("wire: truncated ALPS ACCEPT_CH")
					}
					body = body[2+int(binary.BigEndian.Uint16(body)):]
				}
			}
		default:
			if kind <= 9 {
				return nil, fmt.Errorf("wire: forbidden ALPS frame %d", kind)
			}
		}
	}

	// Native HTTP/2 accepts ENABLE_CONNECT_PROTOCOL only in its first frame.
	// Keep standard settings first even when ALPS contains many extension IDs.
	slices.SortFunc(settings, func(a, b Setting) int { return int(a.ID) - int(b.ID) })

	return settings, nil
}
