package mimic

import (
	"crypto/rand"
	"encoding/binary"
	"slices"

	utls "github.com/refraction-networking/utls"
)

// updateChromiumTLS adds profile changes not yet provided by uTLS.
// Remove the corresponding adjustments when switching to upstream profiles.
func updateChromiumTLS(spec *utls.ClientHelloSpec, majorVersion int) {
	if majorVersion < 150 {
		return
	}

	for _, extension := range spec.Extensions {
		if signatures, ok := extension.(*utls.SignatureAlgorithmsExtension); ok {
			// Advertisement only; uTLS cannot verify ML-DSA signatures yet.
			algorithms := []utls.SignatureScheme{0x0904, 0x0905, 0x0906} // ML-DSA-44/65/87
			if majorVersion >= 152 {
				algorithms = slices.Insert(algorithms, 0, chromiumGREASESignature())
			}
			signatures.SupportedSignatureAlgorithms = append(algorithms, signatures.SupportedSignatureAlgorithms...)
		}
	}

	if majorVersion >= 152 {
		// Keep the trailing GREASE extension in place before shuffling.
		spec.Extensions = slices.Insert(spec.Extensions, len(spec.Extensions)-1, utls.TLSExtension(&utls.GenericExtension{
			Id:   0xca34,
			Data: chromiumTrustAnchors(),
		}))
		spec.Extensions = utls.ShuffleChromeTLSExtensions(spec.Extensions)
	}
}

// uTLS does not replace GREASE placeholders in signature algorithms yet.
func chromiumGREASESignature() utls.SignatureScheme {
	var random [1]byte
	rand.Read(random[:])
	grease := uint16(random[0]&0xf0) | 0x0a

	return utls.SignatureScheme(grease<<8 | grease)
}

func chromiumTrustAnchors() []byte {
	data := make([]byte, 2, 2+len(chromiumTrustAnchorIDs))
	binary.BigEndian.PutUint16(data, uint16(len(chromiumTrustAnchorIDs)))

	return append(data, chromiumTrustAnchorIDs...)
}

// Chromium's non-MTC trust-anchor IDs, each prefixed by its one-byte length.
const chromiumTrustAnchorIDs = "\x05\x82\xdf\x13\x02\x01" +
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
