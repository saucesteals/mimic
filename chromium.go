package mimic

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	utls "github.com/refraction-networking/utls"
	"github.com/saucesteals/fhttp/http2"
)

var (
	ErrUnsupportedVersion = errors.New("mimic: unsupported version")
)

func getMajor(version string) (string, int, error) {
	major := strings.SplitN(version, ".", 2)[0]
	iMajor, err := strconv.Atoi(major)
	return major, iMajor, err
}

func chromiumVersionHelloId(majorVersion int) utls.ClientHelloID {
	switch {
	case majorVersion < 102:
		return utls.HelloChrome_100
	case majorVersion < 106:
		return utls.HelloChrome_102
	case majorVersion < 112:
		return utls.HelloChrome_106_Shuffle
	case majorVersion < 114:
		return utls.HelloChrome_112_PSK_Shuf
	case majorVersion < 115:
		return utls.HelloChrome_114_Padding_PSK_Shuf
	case majorVersion < 120:
		return utls.HelloChrome_115_PQ
	case majorVersion < 131:
		return utls.HelloChrome_120
	case majorVersion < 133:
		return utls.HelloChrome_131
	default: // >=133
		return utls.HelloChrome_133
	}
}

func chromiumVersionHTTP2Options(majorVersion int) *HTTP2Options {
	switch {
	case majorVersion < 107:
		return &HTTP2Options{
			PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
			MaxHeaderListSize: 262144,
			Settings: []http2.Setting{
				{ID: http2.SettingHeaderTableSize, Val: 65536},
				{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
				{ID: http2.SettingInitialWindowSize, Val: 6291456},
				{ID: http2.SettingMaxHeaderListSize, Val: 100000},
			},
			InitialWindowSize: 6291456,
			HeaderTableSize:   65536,
		}
	case majorVersion < 110:
		return &HTTP2Options{
			PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
			MaxHeaderListSize: 262144,
			Settings: []http2.Setting{
				{ID: http2.SettingHeaderTableSize, Val: 65536},
				{ID: http2.SettingEnablePush, Val: 0},
				{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
				{ID: http2.SettingInitialWindowSize, Val: 6291456},
				{ID: http2.SettingMaxHeaderListSize, Val: 262144},
			},
			InitialWindowSize: 6291456,
			HeaderTableSize:   65536,
		}
	case majorVersion < 120:
		return &HTTP2Options{
			PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
			MaxHeaderListSize: 262144,
			Settings: []http2.Setting{
				{ID: http2.SettingHeaderTableSize, Val: 65536},
				{ID: http2.SettingEnablePush, Val: 0},
				{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
				{ID: http2.SettingInitialWindowSize, Val: 6291456},
				{ID: http2.SettingMaxHeaderListSize, Val: 262144},
			},
			InitialWindowSize: 6291456,
			HeaderTableSize:   65536,
		}
	default: // >=120
		return &HTTP2Options{
			PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
			MaxHeaderListSize: 262144,
			Settings: []http2.Setting{
				{ID: http2.SettingHeaderTableSize, Val: 65536},
				{ID: http2.SettingEnablePush, Val: 0},
				{ID: http2.SettingInitialWindowSize, Val: 6291456},
				{ID: http2.SettingMaxHeaderListSize, Val: 262144},
			},
			InitialWindowSize: 6291456,
			HeaderTableSize:   65536,
		}
	}
}

func Chromium(brand Brand, version string) (*ClientSpec, error) {
	majorVersion, majorVersionNumber, err := getMajor(version)
	if err != nil {
		return nil, err
	}

	if majorVersionNumber < 100 {
		return nil, ErrUnsupportedVersion
	}

	helloId := chromiumVersionHelloId(majorVersionNumber)
	getSpec := func() *utls.ClientHelloSpec {
		spec, err := utls.UTLSIdToSpec(helloId)
		if err != nil {
			panic(fmt.Sprintf("mimic: unexpected uid to spec conversion: %v", err)) // this should never happen as we only use built-in hello ids
		}
		return &spec
	}

	return &ClientSpec{
		version,
		clientHintUA(brand, majorVersion, majorVersionNumber),
		chromiumVersionHTTP2Options(majorVersionNumber),
		getSpec,
	}, nil
}
