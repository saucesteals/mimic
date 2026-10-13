// Copyright 2026 The Mimic Authors. All rights reserved.
// Use of this source code is governed by the license in the Mimic repository.

package wire

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	"slices"

	tlsx "github.com/saucesteals/tlsx"
)

// ClientHelloProfile describes client wire choices without replacing TLS state.
// All keys, binders, transcripts and verification remain native. The profile and
// its slices must remain immutable after being attached to a Config.
type ClientHelloProfile struct {
	CipherSuites                []uint16
	Groups                      []tlsx.CurveID
	SignatureSchemes            []tlsx.SignatureScheme
	CertificateSignatureSchemes []tlsx.SignatureScheme
	Extensions                  []ClientHelloExtension
	KeyShares                   []tlsx.CurveID
	ReuseHybridKey              bool
	GreaseKeyShare              tlsx.CurveID
	GreaseVersion               uint16
	// Padding pads a 256..511-byte ClientHello to 512 bytes when extension 21 is present.
	Padding             bool
	ApplicationSettings map[string][]byte
	// PeerApplicationSettings receives authenticated ALPS before the client flight.
	// The callback must validate and apply settings for the negotiated protocol.
	PeerApplicationSettings func(protocol string, settings []byte) error
	// DecompressCertificate must enforce the supplied uncompressed length limit.
	DecompressCertificate func(algorithm uint16, compressed []byte, length int) ([]byte, error)
}

// ClientHelloExtension places a native extension in wire order when Data is nil.
// Non-nil Data supplies an opaque extension body. Dynamic native extensions
// (keys, PSKs, versions, signatures and names) cannot be replaced with raw data.
type ClientHelloExtension struct {
	ID   uint16
	Data []byte
}

func mimicGrease(id uint16) bool { return id&0x0f0f == 0x0a0a && byte(id) == byte(id>>8) }

// Compatibility creates connection-local callbacks. The profile is immutable.
func (p *ClientHelloProfile) Compatibility() (*tlsx.ClientCompatibility, error) {
	if len(p.CipherSuites) == 0 || len(p.Groups) == 0 || len(p.SignatureSchemes) == 0 || len(p.KeyShares) == 0 {
		return nil, errors.New("wire: incomplete profile")
	}
	seen := make(map[uint16]bool)
	for _, ext := range p.Extensions {
		if seen[ext.ID] {
			return nil, errors.New("wire: duplicate extension")
		}
		seen[ext.ID] = true
		if ext.Data != nil {
			switch ext.ID {
			case 0, 5, 10, 11, 13, 16, 18, 23, 35, 41, 43, 44, 45, 50, 51, 65281:
				return nil, errors.New("wire: opaque native extension")
			}
		}
	}
	for _, id := range []uint16{10, 13, 43, 51} {
		if !seen[id] {
			return nil, errors.New("wire: missing required extension")
		}
	}
	if p.GreaseVersion != 0 && !mimicGrease(p.GreaseVersion) ||
		p.GreaseKeyShare != 0 && !mimicGrease(uint16(p.GreaseKeyShare)) {
		return nil, errors.New("wire: invalid GREASE")
	}
	state := &handshakeState{profile: p}
	for _, ext := range p.Extensions {
		switch ext.ID {
		case 28:
			// Native Go already enforces this receive limit. Lower advertised
			// limits would require an additional incoming record constraint.
			if !slices.Equal(ext.Data, []byte{0x40, 1}) {
				return nil, errors.New("wire: record receive limit must be 16385")
			}
			state.recordLimit = true
		case 34:
			if len(ext.Data) < 4 || int(binary.BigEndian.Uint16(ext.Data)) != len(ext.Data)-2 || len(ext.Data)%2 != 0 {
				return nil, errors.New("wire: invalid delegated credential schemes")
			}
			for i := 2; i < len(ext.Data); i += 2 {
				state.delegatedSchemes = append(state.delegatedSchemes, tlsx.SignatureScheme(binary.BigEndian.Uint16(ext.Data[i:])))
			}
		}
	}
	compatibility := &tlsx.ClientCompatibility{
		Hello: tlsx.ClientHelloParameters{
			CipherSuites:                slices.Clone(p.CipherSuites),
			Groups:                      slices.Clone(p.Groups),
			SignatureSchemes:            slices.Clone(p.SignatureSchemes),
			CertificateSignatureSchemes: slices.Clone(p.CertificateSignatureSchemes),
			KeyShares:                   slices.Clone(p.KeyShares),
			ReuseHybridKey:              p.ReuseHybridKey,
			TicketSupported:             seen[35],
		},
		ClientHello:     p.marshalClientHello,
		DecodeHandshake: state.decode,
		PeerPublicKey:   state.verifyCredential,
		LoadSession:     state.loadSession,
		StoreSession:    state.storeSession,
		ClientFlight: func(protocol string) ([]byte, error) {
			if state.alps == 0 {
				return nil, nil
			}
			if p.PeerApplicationSettings == nil {
				return nil, errors.New("wire: negotiated application settings require a handler")
			}
			if err := p.PeerApplicationSettings(protocol, state.peerSettings); err != nil {
				return nil, err
			}
			return marshalSettings(state.alps, p.ApplicationSettings[protocol])
		},
	}
	if seen[27] {
		compatibility.CertificateMessageTypes = []uint8{25}
	}
	if seen[45] {
		compatibility.Hello.PSKModes = []byte{1}
	}
	if state.recordLimit {
		compatibility.RecordSizeLimit = func(uint16) int { return int(state.peerRecordLimit.Load()) }
	}
	return compatibility, nil
}

func (p *ClientHelloProfile) marshalClientHello(raw []byte, inner bool) ([]byte, error) {
	// The native serializer owns the fixed fields; replace only its extension block.
	if len(raw) < 39 {
		return nil, errors.New("wire: short ClientHello")
	}
	pos := 39 + int(raw[38])
	if pos+2 > len(raw) {
		return nil, errors.New("wire: malformed ClientHello")
	}
	pos += 2 + int(binary.BigEndian.Uint16(raw[pos:]))
	if pos >= len(raw) {
		return nil, errors.New("wire: malformed ClientHello")
	}
	pos += 1 + int(raw[pos])
	if pos+2 > len(raw) {
		return nil, errors.New("wire: malformed extensions")
	}
	extensions, err := p.mimicExtensions(raw[pos:], pos+2, inner)
	if err != nil {
		return nil, err
	}
	if len(extensions) > 65535 {
		return nil, errors.New("wire: extension block too large")
	}
	out := append([]byte(nil), raw[:pos]...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(extensions)))
	out = append(out, extensions...)
	size := len(out) - 4
	out[1], out[2], out[3] = byte(size>>16), byte(size>>8), byte(size)
	return out, nil
}

// mimicExtensions reorders the native serialization, not transport writes. Every
// transcript, ECH computation and PSK binder sees these same bytes. The profile's
// randomized order is fixed once per connection, never once per marshal call.
func (p *ClientHelloProfile) mimicExtensions(raw []byte, fixedLength int, inner bool) ([]byte, error) {
	native, err := handshakeExtensions(raw)
	if err != nil {
		return nil, err
	}
	if data, ok := native[43]; ok && len(data) > 0 && p.GreaseVersion != 0 {
		versions := binary.BigEndian.AppendUint16(nil, p.GreaseVersion)
		versions = append(versions, data[1:]...)
		native[43] = append([]byte{byte(len(versions))}, versions...)
	}
	if data, ok := native[51]; ok && len(data) >= 4 &&
		binary.BigEndian.Uint16(data[2:]) == uint16(p.KeyShares[0]) && p.GreaseKeyShare != 0 {
		grease := binary.BigEndian.AppendUint16(nil, uint16(p.GreaseKeyShare))
		grease = append(grease, 0, 1, 0)
		body := append(grease, data[2:]...)
		native[51] = append(binary.BigEndian.AppendUint16(nil, uint16(len(body))), body...)
	}
	var out []byte
	add := func(id uint16, data []byte) error {
		if len(data) > 65535 {
			return errors.New("tls: wire profile extension too large")
		}
		out = binary.BigEndian.AppendUint16(out, id)
		out = binary.BigEndian.AppendUint16(out, uint16(len(data)))
		out = append(out, data...)
		return nil
	}
	for _, ext := range p.Extensions {
		if ext.ID == 41 || ext.ID == 44 || ext.ID == 21 {
			continue
		}
		data, present := native[ext.ID]
		if !present && ext.Data != nil {
			// Real ECH uses its own authenticated extension; GREASE ECH is outer-only.
			if inner && ext.ID == 65037 {
				continue
			}
			data, present = ext.Data, true
		}
		if present {
			if err := add(ext.ID, data); err != nil {
				return nil, err
			}
		}
	}
	if cookie, ok := native[44]; ok {
		if err := add(44, cookie); err != nil {
			return nil, err
		}
	}
	if p.Padding {
		// Include the native PSK extension when computing the final hello length.
		size := fixedLength + len(out)
		if psk, ok := native[41]; ok {
			size += 4 + len(psk)
		}
		if size > 255 && size < 512 {
			if err := add(21, make([]byte, max(1, 512-size-4))); err != nil {
				return nil, err
			}
		}
	}
	if psk, ok := native[41]; ok {
		if err := add(41, psk); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func (p *ClientHelloProfile) offersALPS(codepoint uint16, protocol string) bool {
	if protocol == "" {
		return false
	}
	for _, ext := range p.Extensions {
		if ext.ID != codepoint || len(ext.Data) < 2 {
			continue
		}
		data := ext.Data
		if int(binary.BigEndian.Uint16(data)) != len(data)-2 {
			return false
		}
		for data = data[2:]; len(data) > 0; {
			n := int(data[0])
			data = data[1:]
			if n == 0 || n > len(data) {
				return false
			}
			if string(data[:n]) == protocol {
				return true
			}
			data = data[n:]
		}
	}

	return false
}

func marshalSettings(codepoint uint16, settings []byte) ([]byte, error) {
	if len(settings) > 65529 {
		return nil, errors.New("tls: application settings too large")
	}
	n := 6 + len(settings)
	data := []byte{8, byte(n >> 16), byte(n >> 8), byte(n)}
	data = binary.BigEndian.AppendUint16(data, uint16(4+len(settings)))
	data = binary.BigEndian.AppendUint16(data, codepoint)
	data = binary.BigEndian.AppendUint16(data, uint16(len(settings)))

	return append(data, settings...), nil
}

// GREASE returns a random RFC 8701 reserved value.
func GREASE() uint16 {
	var b [1]byte
	rand.Read(b[:])
	v := uint16(b[0]&15)*16 + 10
	return v<<8 | v
}

// GREASEEncryptedClientHello constructs an outer ECH placeholder. Payload sizes
// include the AEAD tag; AEAD identifiers use the HPKE registry and KDF is SHA256.
// Real ECH always replaces this placeholder through native TLS negotiation.
func GREASEEncryptedClientHello(payloadSizes, aeads []uint16) ([]byte, error) {
	if len(payloadSizes) == 0 || len(aeads) == 0 {
		return nil, errors.New("wire: empty ECH GREASE choices")
	}
	choose := func(values []uint16) (uint16, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(values))))
		if err != nil {
			return 0, err
		}
		return values[n.Int64()], nil
	}
	size, err := choose(payloadSizes)
	if err != nil {
		return nil, err
	}
	aead, err := choose(aeads)
	if err != nil {
		return nil, err
	}
	if size < 16 || (aead != 1 && aead != 2 && aead != 3) {
		return nil, errors.New("wire: invalid ECH GREASE parameters")
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	var id [1]byte
	rand.Read(id[:])
	data := []byte{0, 0, 1, byte(aead >> 8), byte(aead), id[0], 0, 32}
	data = append(data, key.PublicKey().Bytes()...)
	data = binary.BigEndian.AppendUint16(data, size)
	payload := make([]byte, size)
	rand.Read(payload)
	return append(data, payload...), nil
}
