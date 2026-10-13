package wire

import (
	"bytes"
	"compress/zlib"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	tlsx "github.com/saucesteals/tlsx"
	"golang.org/x/crypto/cryptobyte"
)

// DecompressCertificate decodes the RFC 8879 algorithms with bounded output.
// The TLS parser separately checks that the algorithm was actually advertised.
func DecompressCertificate(algorithm uint16, compressed []byte, length int) ([]byte, error) {
	if length < 0 || length > 256<<10 {
		return nil, errors.New("wire: certificate exceeds decompression limit")
	}
	input := bytes.NewReader(compressed)
	var reader io.Reader
	switch algorithm {
	case 1:
		r, err := zlib.NewReader(input)
		if err != nil {
			return nil, err
		}
		defer func() { _ = r.Close() }()
		reader = r
	case 2:
		reader = brotli.NewReader(input)
	case 3:
		r, err := zstd.NewReader(
			input,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderMaxMemory(8<<20),
			zstd.WithDecoderMaxWindow(8<<20),
		)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		reader = r
	default:
		return nil, errors.New("wire: unsupported certificate compression")
	}
	data, err := io.ReadAll(io.LimitReader(reader, int64(length)+1))
	if err != nil {
		return nil, err
	}
	if len(data) != length {
		return nil, errors.New("wire: certificate decompression length mismatch")
	}
	return data, nil
}

func (p *ClientHelloProfile) decompressCertificate(data []byte, limit int) ([]byte, error) {
	if len(data) < 12 || p.DecompressCertificate == nil {
		return nil, errors.New("tls: unexpected compressed certificate")
	}
	alg := binary.BigEndian.Uint16(data[4:6])
	offered := false
	for _, ext := range p.Extensions {
		if ext.ID != 27 || len(ext.Data) < 1 || int(ext.Data[0]) != len(ext.Data)-1 {
			continue
		}
		for i := 1; i+1 < len(ext.Data); i += 2 {
			offered = offered || binary.BigEndian.Uint16(ext.Data[i:]) == alg
		}
	}
	size := int(data[6])<<16 | int(data[7])<<8 | int(data[8])
	compressed := int(data[9])<<16 | int(data[10])<<8 | int(data[11])
	if !offered || size > limit || compressed != len(data)-12 {
		return nil, errors.New("tls: invalid compressed certificate")
	}
	body, err := p.DecompressCertificate(alg, data[12:], size)
	if err != nil || len(body) != size {
		return nil, errors.New("tls: certificate decompression failed")
	}
	raw := append([]byte{11, byte(size >> 16), byte(size >> 8), byte(size)}, body...)
	return raw, nil
}

func (s *handshakeState) readCredential(body []byte) error {
	s.credential = nil
	input := cryptobyte.String(body)
	var context, certificates cryptobyte.String
	if !input.ReadUint8LengthPrefixed(&context) || !input.ReadUint24LengthPrefixed(&certificates) || !input.Empty() {
		return errors.New("wire: malformed Certificate")
	}
	leaf := true
	for !certificates.Empty() {
		var cert, extensions cryptobyte.String
		if !certificates.ReadUint24LengthPrefixed(&cert) || !certificates.ReadUint16LengthPrefixed(&extensions) {
			return errors.New("wire: malformed certificate entry")
		}
		for !extensions.Empty() {
			var id uint16
			var data cryptobyte.String
			if !extensions.ReadUint16(&id) || !extensions.ReadUint16LengthPrefixed(&data) {
				return errors.New("wire: malformed certificate extension")
			}
			if id == 34 {
				if !leaf || s.credential != nil || len(s.delegatedSchemes) == 0 || len(data) == 0 {
					return errors.New("wire: unexpected delegated credential")
				}
				s.credential = slices.Clone([]byte(data))
			}
		}
		leaf = false
	}
	return nil
}

// verifyCredential authenticates the delegated key after native chain and
// hostname verification. Native TLS still verifies CertificateVerify itself.
func (s *handshakeState) verifyCredential(state tlsx.ConnectionState, scheme tlsx.SignatureScheme, now time.Time) (crypto.PublicKey, error) {
	if len(s.credential) == 0 {
		return nil, nil
	}
	raw := cryptobyte.String(s.credential)
	var validTime uint32
	var expected, issuer uint16
	var spki, signature cryptobyte.String
	if !raw.ReadUint32(&validTime) ||
		!raw.ReadUint16(&expected) ||
		!raw.ReadUint24LengthPrefixed(&spki) ||
		!raw.ReadUint16(&issuer) {
		return nil, errors.New("wire: malformed delegated credential")
	}
	signedLength := len(s.credential) - len(raw)
	if !raw.ReadUint16LengthPrefixed(&signature) || !raw.Empty() || len(state.PeerCertificates) == 0 {
		return nil, errors.New("wire: malformed delegated credential signature")
	}
	if scheme != tlsx.SignatureScheme(expected) ||
		!slices.Contains(s.delegatedSchemes, scheme) ||
		!slices.Contains(s.profile.SignatureSchemes, tlsx.SignatureScheme(issuer)) {
		return nil, errors.New("wire: unadvertised delegated credential algorithm")
	}
	leaf := state.PeerCertificates[0]
	expires := leaf.NotBefore.Add(time.Duration(validTime) * time.Second)
	if !expires.After(now) || expires.After(now.Add(7*24*time.Hour)) || !expires.Before(leaf.NotAfter) {
		return nil, errors.New("wire: delegated credential outside validity interval")
	}
	authorized := false
	for _, ext := range leaf.Extensions {
		if ext.Id.Equal(asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 44363, 44}) {
			authorized = !ext.Critical && bytes.Equal(ext.Value, []byte{5, 0})
		}
	}
	if !authorized || leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, errors.New("wire: certificate does not authorize delegation")
	}
	key, err := x509.ParsePKIXPublicKey(spki)
	if err != nil {
		return nil, fmt.Errorf("wire: delegated key: %w", err)
	}
	if _, rsaKey := key.(*rsa.PublicKey); rsaKey {
		return nil, errors.New("wire: RSA delegated keys are unsupported")
	}
	if _, err := credentialSignature(key, scheme); err != nil {
		return nil, err
	}
	algorithm, err := credentialSignature(leaf.PublicKey, tlsx.SignatureScheme(issuer))
	if err != nil {
		return nil, err
	}
	signed := bytes.Repeat([]byte{0x20}, 64)
	signed = append(signed, "TLS, server delegated credentials"...)
	signed = append(signed, 0)
	signed = append(signed, leaf.Raw...)
	signed = append(signed, s.credential[:signedLength]...)
	if err := leaf.CheckSignature(algorithm, signed, signature); err != nil {
		return nil, fmt.Errorf("wire: delegated credential: %w", err)
	}
	return key, nil
}

func credentialSignature(key crypto.PublicKey, scheme tlsx.SignatureScheme) (x509.SignatureAlgorithm, error) {
	switch key := key.(type) {
	case *ecdsa.PublicKey:
		switch {
		case scheme == tlsx.ECDSAWithP256AndSHA256 && key.Curve.Params().BitSize == 256:
			return x509.ECDSAWithSHA256, nil
		case scheme == tlsx.ECDSAWithP384AndSHA384 && key.Curve.Params().BitSize == 384:
			return x509.ECDSAWithSHA384, nil
		case scheme == tlsx.ECDSAWithP521AndSHA512 && key.Curve.Params().BitSize == 521:
			return x509.ECDSAWithSHA512, nil
		}
	case *rsa.PublicKey:
		switch scheme {
		case tlsx.PKCS1WithSHA256:
			return x509.SHA256WithRSA, nil
		case tlsx.PKCS1WithSHA384:
			return x509.SHA384WithRSA, nil
		case tlsx.PKCS1WithSHA512:
			return x509.SHA512WithRSA, nil
		case tlsx.PSSWithSHA256:
			return x509.SHA256WithRSAPSS, nil
		case tlsx.PSSWithSHA384:
			return x509.SHA384WithRSAPSS, nil
		case tlsx.PSSWithSHA512:
			return x509.SHA512WithRSAPSS, nil
		}
	case ed25519.PublicKey:
		if scheme == tlsx.Ed25519 {
			return x509.PureEd25519, nil
		}
	}
	return x509.UnknownSignatureAlgorithm, errors.New("wire: incompatible delegated credential signing key")
}

const credentialSessionPrefix = "mimic:delegated-credential:v1\x00"

func (s *handshakeState) storeSession(session *tlsx.SessionState) {
	if len(s.credential) > 0 {
		session.Extra = append(session.Extra, append([]byte(credentialSessionPrefix), s.credential...))
	}
}

func (s *handshakeState) loadSession(session *tlsx.SessionState, state tlsx.ConnectionState, now time.Time) error {
	s.credential = nil
	for _, extra := range session.Extra {
		if !bytes.HasPrefix(extra, []byte(credentialSessionPrefix)) {
			continue
		}
		if s.credential != nil {
			return errors.New("wire: duplicate cached delegated credential")
		}
		s.credential = slices.Clone(extra[len(credentialSessionPrefix):])
		if len(s.credential) < 6 {
			return errors.New("wire: malformed cached delegated credential")
		}
		scheme := tlsx.SignatureScheme(uint16(s.credential[4])<<8 | uint16(s.credential[5]))
		if _, err := s.verifyCredential(state, scheme, now); err != nil {
			s.credential = nil
			return err
		}
	}
	return nil
}
