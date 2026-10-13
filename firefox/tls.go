package firefox

import (
	tlsx "github.com/saucesteals/tlsx"

	"github.com/saucesteals/mimic/wire"
)

func clientHello() (*wire.ClientHelloProfile, error) {
	ech, err := wire.GREASEEncryptedClientHello([]uint16{239}, []uint16{1, 3})
	if err != nil {
		return nil, err
	}
	return &wire.ClientHelloProfile{
		CipherSuites: []uint16{
			0x1301, 0x1303, 0x1302, 0xc02b, 0xc02f, 0xcca9,
			0xcca8, 0xc02c, 0xc030, 0xc013, 0xc014, 0x009c,
			0x009d, 0x002f, 0x0035,
		},
		Groups: []tlsx.CurveID{
			tlsx.X25519MLKEM768,
			tlsx.X25519,
			tlsx.CurveP256,
			tlsx.CurveP384,
			tlsx.CurveP521,
		},
		SignatureSchemes: []tlsx.SignatureScheme{
			tlsx.ECDSAWithP256AndSHA256,
			tlsx.ECDSAWithP384AndSHA384,
			tlsx.ECDSAWithP521AndSHA512,
			tlsx.PSSWithSHA256,
			tlsx.PSSWithSHA384,
			tlsx.PSSWithSHA512,
			tlsx.PKCS1WithSHA256,
			tlsx.PKCS1WithSHA384,
			tlsx.PKCS1WithSHA512,
			tlsx.ECDSAWithSHA1,
			tlsx.PKCS1WithSHA1,
		},
		KeyShares:             []tlsx.CurveID{tlsx.X25519MLKEM768, tlsx.X25519, tlsx.CurveP256},
		ReuseHybridKey:        true,
		DecompressCertificate: wire.DecompressCertificate,
		Extensions: []wire.ClientHelloExtension{
			{ID: 0},
			{ID: 23},
			{ID: 65281},
			{ID: 10},
			{ID: 11},
			{ID: 35},
			{ID: 16},
			{ID: 5},
			{ID: 34, Data: []byte{0, 8, 4, 3, 5, 3, 6, 3, 2, 3}},
			{ID: 18},
			{ID: 51},
			{ID: 43},
			{ID: 13},
			{ID: 45},
			{ID: 28, Data: []byte{0x40, 1}},
			{ID: 27, Data: []byte{6, 0, 1, 0, 2, 0, 3}},
			{ID: 65037, Data: ech},
		},
	}, nil
}
