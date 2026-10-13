package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"

	tlsx "github.com/saucesteals/tlsx"
)

type handshakeState struct {
	profile          *ClientHelloProfile
	version          uint16
	alps             uint16
	peerSettings     []byte
	recordLimit      bool
	peerRecordLimit  atomic.Uint32
	delegatedSchemes []tlsx.SignatureScheme
	credential       []byte
}

func (s *handshakeState) decode(data []byte, limit int) ([]byte, error) {
	if len(data) < 4 {
		return nil, errors.New("wire: short handshake")
	}
	if data[0] == 25 {
		if s.version != tlsx.VersionTLS13 {
			return nil, errors.New("wire: certificate compression requires TLS 1.3")
		}
		var err error
		data, err = s.profile.decompressCertificate(data, limit)
		if err != nil {
			return nil, fmt.Errorf("%v: %w", err, tlsx.AlertError(42))
		}
	}
	if data[0] == 11 && s.version == tlsx.VersionTLS13 {
		if err := s.readCredential(data[4:]); err != nil {
			return nil, fmt.Errorf("%v: %w", err, tlsx.AlertError(47))
		}
	}
	if data[0] == 2 {
		if len(data) < 42 || 39+int(data[38])+3 > len(data) {
			return nil, errors.New("wire: malformed ServerHello")
		}
		s.version = binary.BigEndian.Uint16(data[4:])
		pos := 39 + int(data[38]) + 3
		if pos == len(data) {
			s.credential = nil
			return data, nil
		}
		exts, err := handshakeExtensions(data[pos:])
		if err != nil {
			return nil, err
		}
		if v, ok := exts[43]; ok {
			if len(v) != 2 {
				return nil, errors.New("wire: malformed selected version")
			}
			s.version = binary.BigEndian.Uint16(v)
		}
		if s.version != tlsx.VersionTLS13 {
			s.credential = nil
		}
		if body, ok := exts[28]; ok {
			if s.version == tlsx.VersionTLS13 {
				return nil, errors.New("wire: record limit in TLS 1.3 ServerHello")
			}
			if err := s.setRecordLimit(body); err != nil {
				return nil, err
			}
		}
	}
	if data[0] != 8 {
		return data, nil
	}
	exts, err := handshakeExtensions(data[4:])
	if err != nil {
		return nil, err
	}
	var protocol string
	if body, ok := exts[16]; ok && len(body) >= 3 && int(body[2])+3 == len(body) {
		protocol = string(body[3:])
	}
	for _, id := range []uint16{17513, 17613} {
		if body, ok := exts[id]; ok {
			if s.alps != 0 {
				return nil, errors.New("wire: duplicate ALPS")
			}
			s.alps = id
			s.peerSettings = body
		}
	}
	if s.alps != 0 && !s.profile.offersALPS(s.alps, protocol) {
		return nil, fmt.Errorf("tls: server selected unadvertised application settings: %w", tlsx.AlertError(110))
	}
	if body, ok := exts[28]; ok {
		if err := s.setRecordLimit(body); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func handshakeExtensions(raw []byte) (map[uint16][]byte, error) {
	if len(raw) < 2 || int(binary.BigEndian.Uint16(raw)) != len(raw)-2 {
		return nil, errors.New("wire: malformed extensions")
	}
	exts := make(map[uint16][]byte)
	for raw = raw[2:]; len(raw) > 0; {
		if len(raw) < 4 {
			return nil, errors.New("wire: truncated extension")
		}
		id, n := binary.BigEndian.Uint16(raw), int(binary.BigEndian.Uint16(raw[2:]))
		raw = raw[4:]
		if n > len(raw) {
			return nil, errors.New("wire: truncated extension")
		}
		if _, ok := exts[id]; ok {
			return nil, errors.New("wire: duplicate extension")
		}
		exts[id], raw = raw[:n], raw[n:]
	}
	return exts, nil
}

func (s *handshakeState) setRecordLimit(body []byte) error {
	if !s.recordLimit || len(body) != 2 {
		return errors.New("wire: unsolicited or malformed record limit")
	}
	limit := binary.BigEndian.Uint16(body)
	max := uint16(16384)
	if s.version == tlsx.VersionTLS13 {
		max++
	}
	if limit < 64 || limit > max {
		return fmt.Errorf("wire: invalid record limit: %w", tlsx.AlertError(47))
	}
	s.peerRecordLimit.Store(uint32(limit))
	return nil
}
