package mimic

import (
	"errors"
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

func Chromium(brand Brand, version string) (*ClientSpec, error) {
	majorVersion, majorVersionNumber, err := getMajor(version)

	if err != nil {
		return nil, err
	}

	// legacy
	switch {
	case majorVersionNumber < 100:
		return nil, ErrUnsupportedVersion
	case majorVersionNumber < 107:
		return &ClientSpec{
			version,
			clientHintUA(brand, majorVersion, majorVersionNumber),
			&http2Options{
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
			},
			func() *utls.ClientHelloSpec {
				return &utls.ClientHelloSpec{
					CipherSuites: []uint16{
						utls.GREASE_PLACEHOLDER,
						utls.TLS_AES_128_GCM_SHA256,
						utls.TLS_AES_256_GCM_SHA384,
						utls.TLS_CHACHA20_POLY1305_SHA256,
						utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
						utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
						utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
						utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
						utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
						utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
						utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
						utls.TLS_RSA_WITH_AES_128_CBC_SHA,
						utls.TLS_RSA_WITH_AES_256_CBC_SHA,
					},
					CompressionMethods: []byte{
						0x00, // compressionNone
					},
					Extensions: []utls.TLSExtension{
						&utls.UtlsGREASEExtension{},
						&utls.SNIExtension{},
						&utls.UtlsExtendedMasterSecretExtension{},
						&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
						&utls.SupportedCurvesExtension{
							Curves: []utls.CurveID{
								utls.CurveID(utls.GREASE_PLACEHOLDER),
								utls.X25519,
								utls.CurveP256,
								utls.CurveP384,
							}},
						&utls.SupportedPointsExtension{SupportedPoints: []byte{
							0x00, // pointFormatUncompressed
						}},
						&utls.SessionTicketExtension{},
						&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
						&utls.StatusRequestExtension{},
						&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
							utls.ECDSAWithP256AndSHA256,
							utls.PSSWithSHA256,
							utls.PKCS1WithSHA256,
							utls.ECDSAWithP384AndSHA384,
							utls.PSSWithSHA384,
							utls.PKCS1WithSHA384,
							utls.PSSWithSHA512,
							utls.PKCS1WithSHA512,
						}},
						&utls.SCTExtension{},
						&utls.KeyShareExtension{
							KeyShares: []utls.KeyShare{
								{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
								{Group: utls.X25519},
							}},
						&utls.PSKKeyExchangeModesExtension{
							Modes: []uint8{
								utls.PskModeDHE,
							}},
						&utls.SupportedVersionsExtension{
							Versions: []uint16{
								utls.GREASE_PLACEHOLDER,
								utls.VersionTLS13,
								utls.VersionTLS12,
							}},
						&utls.UtlsCompressCertExtension{
							Algorithms: []utls.CertCompressionAlgo{
								utls.CertCompressionBrotli,
							},
						},
						&utls.ApplicationSettingsExtension{SupportedProtocols: []string{"h2"}},
						&utls.UtlsGREASEExtension{},
						&utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
					},
				}
			},
		}, nil
	}

	// latest
	return &ClientSpec{
		version,
		clientHintUA(brand, majorVersion, majorVersionNumber),
		&http2Options{
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
		},
		func() *utls.ClientHelloSpec {
			return &utls.ClientHelloSpec{
				CipherSuites: []uint16{
					utls.GREASE_PLACEHOLDER,
					utls.TLS_AES_128_GCM_SHA256,
					utls.TLS_AES_256_GCM_SHA384,
					utls.TLS_CHACHA20_POLY1305_SHA256,
					utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					utls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					utls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					utls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					utls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					utls.TLS_RSA_WITH_AES_128_GCM_SHA256,
					utls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					utls.TLS_RSA_WITH_AES_128_CBC_SHA,
					utls.TLS_RSA_WITH_AES_256_CBC_SHA,
				},
				CompressionMethods: []byte{
					0x00, // compressionNone
				},
				Extensions: []utls.TLSExtension{
					&utls.UtlsGREASEExtension{},
					&utls.SNIExtension{},
					&utls.UtlsExtendedMasterSecretExtension{},
					&utls.RenegotiationInfoExtension{Renegotiation: utls.RenegotiateOnceAsClient},
					&utls.SupportedCurvesExtension{
						Curves: []utls.CurveID{
							utls.CurveID(utls.GREASE_PLACEHOLDER),
							utls.X25519,
							utls.CurveP256,
							utls.CurveP384,
						}},
					&utls.SupportedPointsExtension{SupportedPoints: []byte{
						0x00, // pointFormatUncompressed
					}},
					&utls.SessionTicketExtension{},
					&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
					&utls.StatusRequestExtension{},
					&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
						utls.ECDSAWithP256AndSHA256,
						utls.PSSWithSHA256,
						utls.PKCS1WithSHA256,
						utls.ECDSAWithP384AndSHA384,
						utls.PSSWithSHA384,
						utls.PKCS1WithSHA384,
						utls.PSSWithSHA512,
						utls.PKCS1WithSHA512,
					}},
					&utls.SCTExtension{},
					&utls.KeyShareExtension{
						KeyShares: []utls.KeyShare{
							{Group: utls.CurveID(utls.GREASE_PLACEHOLDER), Data: []byte{0}},
							{Group: utls.X25519},
						}},
					&utls.PSKKeyExchangeModesExtension{
						Modes: []uint8{
							utls.PskModeDHE,
						}},
					&utls.SupportedVersionsExtension{
						Versions: []uint16{
							utls.GREASE_PLACEHOLDER,
							utls.VersionTLS13,
							utls.VersionTLS12,
						}},
					&utls.UtlsCompressCertExtension{
						Algorithms: []utls.CertCompressionAlgo{
							utls.CertCompressionBrotli,
						},
					},
					&utls.ApplicationSettingsExtension{SupportedProtocols: []string{"h2"}},
					&utls.UtlsGREASEExtension{},
					&utls.UtlsPaddingExtension{GetPaddingLen: utls.BoringPaddingStyle},
				},
			}
		},
	}, nil
}
