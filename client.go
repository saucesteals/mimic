package mimic

import (
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
}

func (c *ClientSpec) PseudoHeaderOrder() []string {
	return c.h2Options.PseudoHeaderOrder
}
