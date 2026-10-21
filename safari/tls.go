package safari

import (
	"slices"

	tlsx "github.com/saucesteals/tlsx"

	"github.com/saucesteals/mimic/wire"
)

func clientHello(initial bool, platform Platform) (*wire.ClientHelloProfile, error) {
	group, first, last := wire.GREASE(), wire.GREASE(), wire.GREASE()
	if first == last {
		last ^= 0x1010
	}
	p := &wire.ClientHelloProfile{
		CipherSuites: []uint16{
			wire.GREASE(), 0x1302, 0x1303, 0x1301, 0xc02c, 0xc02b,
			0xcca9, 0xc030, 0xc02f, 0xcca8, 0xc00a, 0xc009,
			0xc014, 0xc013, 0x009d, 0x009c, 0x0035, 0x002f,
			0xc008, 0xc012, 0x000a,
		},
		Groups: []tlsx.CurveID{
			tlsx.CurveID(group),
			tlsx.X25519MLKEM768,
			tlsx.X25519,
			tlsx.CurveP256,
			tlsx.CurveP384,
			tlsx.CurveP521,
		},
		SignatureSchemes: []tlsx.SignatureScheme{
			tlsx.ECDSAWithP256AndSHA256,
			tlsx.PSSWithSHA256,
			tlsx.PKCS1WithSHA256,
			tlsx.ECDSAWithP384AndSHA384,
			tlsx.PSSWithSHA384,
			tlsx.PSSWithSHA384,
			tlsx.PKCS1WithSHA384,
			tlsx.PSSWithSHA512,
			tlsx.PKCS1WithSHA512,
			tlsx.PKCS1WithSHA1,
		},
		KeyShares:             []tlsx.CurveID{tlsx.X25519MLKEM768, tlsx.X25519},
		GreaseKeyShare:        tlsx.CurveID(group),
		GreaseVersion:         wire.GREASE(),
		DecompressCertificate: wire.DecompressCertificate,
		Extensions: []wire.ClientHelloExtension{
			{ID: first, Data: []byte{}},
			{ID: 0},
			{ID: 23},
			{ID: 65281},
			{ID: 10},
			{ID: 11},
			{ID: 16},
			{ID: 5},
			{ID: 13},
			{ID: 18},
			{ID: 51},
			{ID: 45},
			{ID: 43},
			{ID: 27, Data: []byte{2, 0, 1}},
			{ID: last, Data: []byte{0}},
		},
	}
	if initial {
		p.Extensions = slices.Insert(p.Extensions, 6, wire.ClientHelloExtension{ID: 35})
		if platform != Mac {
			// The initial iOS 26 snapshot predates the hybrid key-share update.
			copy(p.CipherSuites[1:4], []uint16{0x1301, 0x1302, 0x1303})
			p.Groups = slices.Delete(p.Groups, 1, 2)
			p.KeyShares = []tlsx.CurveID{tlsx.X25519}
			p.Padding = true
			p.Extensions = append(p.Extensions, wire.ClientHelloExtension{ID: 21})
		}
	}

	return p, nil
}
