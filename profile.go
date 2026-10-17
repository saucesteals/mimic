package mimic

import (
	"errors"
	"math"
	"net/http"

	"github.com/saucesteals/mimic/wire"
)

// HeaderOrderKey optionally specifies regular-header order for one request.
// Set req.Header[HeaderOrderKey] to header names in the desired order. Names are
// case-insensitive; comma-separated values are also accepted. Unlisted headers
// follow the profile order, then lexical order. HTTP/2 pseudoheaders retain their
// profile order. Mimic consumes this metadata; it is not sent to the peer.
// The key is intentionally invalid as an HTTP header name: stock net/http
// rejects requests containing it instead of sending the metadata.
const HeaderOrderKey = wire.HeaderOrderKey

// Profile describes browser TLS and HTTP wire behavior.
// Configure it before use; do not mutate it while any transport uses it.
// ClientHello must return a fresh specification for each connection.
type Profile struct {
	ClientHello func() (*wire.ClientHelloProfile, error)
	HTTP2       *HTTP2Profile
	// Headers supplies defaults without replacing caller headers. When the
	// profile supplies Accept-Encoding, the transport decodes supported responses
	// automatically, unless compression is disabled or the request is HEAD/ranged.
	Headers             http.Header
	HeaderOrder         []string
	ResourceHeaderOrder []string
}

// HTTP2Profile describes HTTP/2 serialization and native transport options.
type HTTP2Profile struct {
	Settings          []HTTP2Setting
	PseudoHeaderOrder []string
	// Config supplies native options. Settings owns the decoder table size,
	// initial stream receive window and maximum frame size; their Config fields are ignored.
	Config http.HTTP2Config
	// StreamReceiveWindow optionally enlarges each stream immediately after its
	// initial HEADERS. Zero keeps the window advertised in Settings.
	StreamReceiveWindow int
	// Priority is the native HEADERS tuple; nil omits it, including its flag.
	Priority *HTTP2Priority
}

// nativeConfig derives receive limits from the settings actually sent to peers.
func (p *HTTP2Profile) nativeConfig() (http.HTTP2Config, error) {
	cfg := p.Config
	cfg.MaxDecoderHeaderTableSize = 4096
	cfg.MaxReceiveBufferPerStream = 65535
	cfg.MaxReadFrameSize = 16384
	seen := make(map[uint16]bool)
	for _, setting := range p.Settings {
		if seen[setting.ID] {
			return cfg, errors.New("mimic: duplicate HTTP/2 setting")
		}
		seen[setting.ID] = true
		switch setting.ID {
		case 1:
			cfg.MaxDecoderHeaderTableSize = int(setting.Val)
		case 2:
			if setting.Val != 0 {
				return cfg, errors.New("mimic: HTTP/2 server push is unsupported")
			}
		case 4:
			cfg.MaxReceiveBufferPerStream = int(setting.Val)
		case 5:
			cfg.MaxReadFrameSize = int(setting.Val)
		}
	}
	if !seen[2] {
		return cfg, errors.New("mimic: HTTP/2 profiles must disable server push")
	}
	if p.StreamReceiveWindow != 0 {
		if p.StreamReceiveWindow < cfg.MaxReceiveBufferPerStream || p.StreamReceiveWindow > math.MaxInt32 {
			return cfg, errors.New("mimic: stream receive window cannot shrink or exceed 31 bits")
		}
		cfg.MaxReceiveBufferPerStream = p.StreamReceiveWindow
	}
	// net/http replaces out-of-range values with defaults, which would disagree
	// with our wire settings. Reject limits its public configuration cannot express.
	if cfg.MaxDecoderHeaderTableSize < 1 || cfg.MaxDecoderHeaderTableSize > math.MaxInt32 ||
		cfg.MaxReceiveBufferPerStream < 1 || cfg.MaxReceiveBufferPerStream > math.MaxInt32 ||
		cfg.MaxReadFrameSize < 16384 || cfg.MaxReadFrameSize > 1<<24-1 {
		return cfg, errors.New("mimic: HTTP/2 settings exceed native configuration limits")
	}
	if cfg.MaxEncoderHeaderTableSize < 1 || cfg.MaxEncoderHeaderTableSize > math.MaxInt32 {
		cfg.MaxEncoderHeaderTableSize = 4096
	}

	return cfg, nil
}

// orderedHeaders selects navigation or subresource header ordering.
func (p *Profile) orderedHeaders(fetchMode string) []string {
	if len(p.ResourceHeaderOrder) != 0 && fetchMode != "" && fetchMode != "navigate" {
		return p.ResourceHeaderOrder
	}

	return p.HeaderOrder
}

// HTTP2Setting is a setting identifier and value in wire order.
type HTTP2Setting = wire.Setting

// HTTP2Priority is the HTTP/2 HEADERS priority tuple.
type HTTP2Priority = wire.Priority

func (p *HTTP2Profile) streamWindowIncrement() uint32 {
	if p.StreamReceiveWindow == 0 {
		return 0
	}
	initial := uint32(65535)
	for _, setting := range p.Settings {
		if setting.ID == 4 {
			initial = setting.Val
		}
	}
	return uint32(p.StreamReceiveWindow) - initial
}
