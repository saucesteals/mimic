package mimic

import (
	"log"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"

	"github.com/saucesteals/fhttp/http2"
)

type Platform string

const (
	PlatformWindows Platform = "win"
	PlatformMac     Platform = "mac"
	PlatformLinux   Platform = "linux"
)

type Brand string

const (
	BrandChrome Brand = "Google Chrome"
	BrandBrave  Brand = "Brave"
	BrandEdge   Brand = "Microsoft Edge"
)

type ClientSpec struct {
	version      string
	clientHintUA string
	HTTP2Options *HTTP2Options
	GetTlsSpec   func() *utls.ClientHelloSpec
}

type HTTP2Options struct {
	Settings          []http2.Setting
	PseudoHeaderOrder []string
	MaxHeaderListSize uint32
	InitialWindowSize uint32
	HeaderTableSize   uint32
}

// ConfigureTransport configures a http.Transport to follow the client's spec
// Returns the given Transport for convenience
func (c *ClientSpec) ConfigureTransport(t1 *http.Transport) *http.Transport {
	t1.GetTlsClientHelloSpec = c.GetTlsSpec

	t2, err := http2.ConfigureTransports(t1)

	if err != nil {
		log.Printf("mimic: error enabling Transport HTTP/2 support: %v", err)
		return t1
	}

	t2.Settings = c.HTTP2Options.Settings
	t2.MaxHeaderListSize = c.HTTP2Options.MaxHeaderListSize
	t2.InitialWindowSize = c.HTTP2Options.InitialWindowSize
	t2.HeaderTableSize = c.HTTP2Options.HeaderTableSize

	return t1
}

// Version returns the version for the mimicked client..
func (c *ClientSpec) Version() string {
	return c.version
}

// ClientHintUA returns the "sec-ch-ua" header value for the mimicked client.
func (c *ClientSpec) ClientHintUA() string {
	return c.clientHintUA
}

// PseudoHeaderOrder returns the pseudo header order for the mimicked client.
func (c *ClientSpec) PseudoHeaderOrder() []string {
	return c.HTTP2Options.PseudoHeaderOrder
}
