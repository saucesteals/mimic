package chromium

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"slices"

	tlsx "github.com/saucesteals/tlsx"

	"github.com/saucesteals/mimic/wire"
)

func clientHello(major int, platform Platform) (*wire.ClientHelloProfile, error) {
	// Android enabled post-quantum key agreement by default in Chromium 133.
	hybrid := major >= 131 && (platform != Android || major >= 133)
	group, first, last := wire.GREASE(), wire.GREASE(), wire.GREASE()
	if first == last {
		last ^= 0x1010
	}
	p := &wire.ClientHelloProfile{
		CipherSuites: []uint16{
			wire.GREASE(), 0x1301, 0x1302, 0x1303, 0xc02b, 0xc02f,
			0xc02c, 0xc030, 0xcca9, 0xcca8, 0xc013, 0xc014,
			0x009c, 0x009d, 0x002f, 0x0035,
		},
		Groups: []tlsx.CurveID{
			tlsx.CurveID(group),
			tlsx.X25519,
			tlsx.CurveP256,
			tlsx.CurveP384,
		},
		SignatureSchemes: []tlsx.SignatureScheme{
			tlsx.ECDSAWithP256AndSHA256,
			tlsx.PSSWithSHA256,
			tlsx.PKCS1WithSHA256,
			tlsx.ECDSAWithP384AndSHA384,
			tlsx.PSSWithSHA384,
			tlsx.PKCS1WithSHA384,
			tlsx.PSSWithSHA512,
			tlsx.PKCS1WithSHA512,
		},
		KeyShares:             []tlsx.CurveID{tlsx.X25519},
		GreaseKeyShare:        tlsx.CurveID(group),
		GreaseVersion:         wire.GREASE(),
		Padding:               !hybrid,
		DecompressCertificate: wire.DecompressCertificate,
	}
	if hybrid {
		p.KeyShares = []tlsx.CurveID{tlsx.X25519MLKEM768, tlsx.X25519}
		p.Groups = slices.Insert(p.Groups, 1, tlsx.X25519MLKEM768)
	}
	if major >= 150 {
		p.SignatureSchemes = append([]tlsx.SignatureScheme{tlsx.MLDSA44, tlsx.MLDSA65, tlsx.MLDSA87}, p.SignatureSchemes...)
	}
	if major >= 152 {
		p.SignatureSchemes = slices.Insert(p.SignatureSchemes, 0, tlsx.SignatureScheme(wire.GREASE()))
	}
	alps := uint16(17513)
	if major >= 133 {
		alps = 17613
	}
	middle := []wire.ClientHelloExtension{
		{ID: 0},
		{ID: 23},
		{ID: 65281},
		{ID: 10},
		{ID: 11},
		{ID: 35},
		{ID: 16},
		{ID: 5},
		{ID: 13},
		{ID: 18},
		{ID: 51},
		{ID: 45},
		{ID: 43},
		{ID: 27, Data: []byte{2, 0, 2}},
		{ID: alps, Data: []byte{0, 3, 2, 'h', '2'}},
	}
	if major >= 120 {
		ech, err := wire.GREASEEncryptedClientHello([]uint16{144, 176, 208, 240}, []uint16{1})
		if err != nil {
			return nil, err
		}
		middle = append(middle, wire.ClientHelloExtension{ID: 65037, Data: ech})
	}
	if major >= 152 {
		middle = append(middle, wire.ClientHelloExtension{ID: 0xca34, Data: trustAnchors()})
	}
	// Use the fixed pre-rollout order for 107–109; permutation is standard from 110.
	if major >= 110 {
		for i := len(middle) - 1; i > 0; i-- {
			j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
			if err != nil {
				return nil, err
			}
			middle[i], middle[j.Int64()] = middle[j.Int64()], middle[i]
		}
	}
	p.Extensions = append([]wire.ClientHelloExtension{{ID: first, Data: []byte{}}}, middle...)
	p.Extensions = append(p.Extensions, wire.ClientHelloExtension{ID: last, Data: []byte{0}})
	if p.Padding {
		p.Extensions = append(p.Extensions, wire.ClientHelloExtension{ID: 21})
	}

	return p, nil
}

func trustAnchors() []byte {
	data := make([]byte, 2, 2+len(trustAnchorIDs))
	binary.BigEndian.PutUint16(data, uint16(len(trustAnchorIDs)))

	return append(data, trustAnchorIDs...)
}

// Chromium's non-MTC trust-anchor IDs, each prefixed by its one-byte length.
const trustAnchorIDs = "\x05\x82\xdf\x13\x02\x01" +
	"\x05\x82\xdf\x13\x02\x06" +
	"\x05\x82\xdf\x13\x02\x0d" +
	"\x05\x82\xdf\x13\x02\x0e" +
	"\x05\x82\xdf\x13\x02\x0f" +
	"\x05\x82\xdf\x13\x02\x12" +
	"\x05\x82\xdf\x13\x02\x13" +
	"\x05\x82\xdf\x13\x02\x14" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x07" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x08" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x09" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x0a" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x0b" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x0c" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x0d" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x12" +
	"\x08\x83\x9a\x64\x8c\x9b\x2d\x01\x13" +
	"\x04\xd6\x79\x09\x01" +
	"\x04\xd6\x79\x09\x04" +
	"\x04\xd6\x79\x09\x05" +
	"\x04\xd6\x79\x09\x06" +
	"\x04\xd6\x79\x09\x07" +
	"\x04\xd6\x79\x09\x08" +
	"\x04\xd6\x79\x09\x0a" +
	"\x04\xd6\x79\x09\x0b" +
	"\x04\xd6\x79\x09\x0c" +
	"\x04\xd6\x79\x09\x0d" +
	"\x04\xd6\x79\x09\x0f"
