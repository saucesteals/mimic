package mimic

import (
	"log"

	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

// ConfigureTransport configures a net/http HTTP/1 Transport to follow the client's spec
func (c *ClientSpec) ConfigureTransport(t1 *http.Transport) {
	t1.GetTlsClientHelloSpec = c.GetTLSSpec

	t2, err := http2.ConfigureTransports(t1)

	if err != nil {
		log.Printf("error enabling Transport HTTP/2 support: %v", err)
		return
	}

	t2.Settings = c.h2Options.Settings
	t2.MaxHeaderListSize = c.h2Options.MaxHeaderListSize
	t2.InitialWindowSize = c.h2Options.MaxHeaderListSize
	t2.HeaderTableSize = c.h2Options.MaxHeaderListSize
}
