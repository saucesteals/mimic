package mimic

import (
	"log"

	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

type Transport struct {
	T    *http.Transport
	spec *ClientSpec
}

func (c *ClientSpec) NewTransport(t1 *http.Transport) *Transport {
	if t1 == nil {
		t1 = &http.Transport{}
	}

	t := &Transport{T: t1, spec: c}
	t.T.TlsClientHelloSpec = c.GetTLSSpec()

	t2, err := http2.ConfigureTransports(t.T)

	if err != nil {
		log.Printf("error enabling Transport HTTP/2 support: %v", err)
		return t
	}

	t2.Settings = c.h2Options.Settings
	t2.MaxHeaderListSize = c.h2Options.MaxHeaderListSize
	t2.InitialWindowSize = c.h2Options.MaxHeaderListSize
	t2.HeaderTableSize = c.h2Options.MaxHeaderListSize

	return t

}
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, values := range t.spec.headers {
		if _, ok := req.Header[key]; !ok {
			req.Header[key] = values
		}
	}

	return t.T.RoundTrip(req)
}
