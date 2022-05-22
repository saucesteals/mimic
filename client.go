package mimic

import (
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

var (
	chromeH2Options = &H2Options{
		PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
		MaxHeaderListSize: 262144,
		Settings: []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 65536},
			{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
			{ID: http2.SettingInitialWindowSize, Val: 6291456},
			{ID: http2.SettingMaxHeaderListSize, Val: 262144},
		},
		InitialWindowSize: 6291456,
		HeaderTableSize:   65536,
	}

	Chrome101 = &ClientSpec{
		headers: http.Header{
			"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"},
			"accept-encoding":           {"gzip, deflate, br"},
			"accept-language":           {"en-US,en;q=0.9"},
			"sec-ch-ua":                 {`" Not A;Brand";v="99", "Chromium";v="101", "Google Chrome";v="101"`},
			"sec-ch-ua-mobile":          {"?0"},
			"sec-ch-ua-platform":        {`"macOS"`},
			"sec-fetch-dest":            {"document"},
			"sec-fetch-mode":            {"navigate"},
			"sec-fetch-site":            {"none"},
			"sec-fetch-user":            {"?1"},
			"upgrade-insecure-requests": {"1"},
			"user-agent":                {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.64 Safari/537.36"},
		},
		h2Options: chromeH2Options,
	}
)

type H2Options struct {
	Settings          []http2.Setting
	PseudoHeaderOrder []string
	MaxHeaderListSize uint32
	InitialWindowSize uint32
	HeaderTableSize   uint32
}

type ClientSpec struct {
	h2Options *H2Options
	headers   http.Header
}

func (c *ClientSpec) PseudoHeaderOrder() []string {
	return c.h2Options.PseudoHeaderOrder
}
