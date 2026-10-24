package janus

import (
	"errors"

	"google.golang.org/protobuf/encoding/protowire"
)

// LoginEnvelope is the recovered o070 response envelope. Application status is
// field 1; fields 2 through 13 form a oneof carrying the status-specific
// payload; field 14 is an optional byte-array side channel.
type LoginEnvelope struct {
	Status     uint64
	PayloadTag protowire.Number
	Payload    []byte
	Extra      []byte
}

// ParseLoginEnvelope parses the o070 response. It rejects malformed bytes and
// a success status (1) whose payload is not present, matching the bridge's
// fail-closed policy. Transport success alone never establishes a session.
func ParseLoginEnvelope(data []byte) (LoginEnvelope, error) {
	var envelope LoginEnvelope
	invalid := errors.New("invalid Janus login response protobuf")
	for len(data) > 0 {
		number, wireType, tagSize := protowire.ConsumeTag(data)
		if tagSize < 0 {
			return LoginEnvelope{}, invalid
		}
		data = data[tagSize:]
		switch {
		case number == 1:
			if wireType != protowire.VarintType {
				return LoginEnvelope{}, invalid
			}
			status, size := protowire.ConsumeVarint(data)
			if size < 0 || status > 1<<31-1 {
				return LoginEnvelope{}, invalid
			}
			envelope.Status = status
			data = data[size:]
		case number >= 2 && number <= 13:
			if wireType != protowire.BytesType {
				return LoginEnvelope{}, invalid
			}
			payload, size := protowire.ConsumeBytes(data)
			if size < 0 {
				return LoginEnvelope{}, invalid
			}
			envelope.PayloadTag = number
			envelope.Payload = append([]byte(nil), payload...)
			data = data[size:]
		case number == 14:
			if wireType != protowire.BytesType {
				return LoginEnvelope{}, invalid
			}
			extra, size := protowire.ConsumeBytes(data)
			if size < 0 {
				return LoginEnvelope{}, invalid
			}
			envelope.Extra = append([]byte(nil), extra...)
			data = data[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, data)
			if size < 0 {
				return LoginEnvelope{}, invalid
			}
			data = data[size:]
		}
	}
	if envelope.Status == 0 || (envelope.Status == 1 && (envelope.PayloadTag != 2 || len(envelope.Payload) == 0)) {
		return LoginEnvelope{}, errors.New("Janus login response is missing its application status or success payload")
	}
	return envelope, nil
}

// StatusName labels the recovered application-status branches. It is not an
// exhaustive enum dictionary; unknown statuses are reported verbatim.
func (envelope LoginEnvelope) StatusName() string {
	switch envelope.Status {
	case 1:
		return "success"
	case 2, 3:
		return "challenge"
	case 7, 9, 17, 18:
		return "integrity_retry"
	case 21:
		return "result_with_metadata"
	default:
		return "unknown"
	}
}

// ScopedToken is one ivs0 entry: a token string, its scope string, and the
// long-valued timing field (expiry/lifetime semantics are not fully verified).
type ScopedToken struct {
	Token  string
	Scope  string
	Expiry int64
}

// Session is the decoded success payload: o070 field 2 -> h18 field 1 -> b841,
// whose field 6 (ffu0) carries the primary string and repeated scoped tokens.
type Session struct {
	// b841 fields 1-5, identity strings with unverified semantics.
	Identity []string
	// ffu0 fields.
	Primary      string // ffu0 field 1
	ScopedTokens []ScopedToken
	ContainerInt int64  // ffu0 field 3
	ContainerStr string // ffu0 field 5
	// h18 byte fields 2, 3, 4, 11, retained raw.
	SuccessBytes [][]byte
}

// DecodeSuccess walks the recovered success nesting and extracts the identity
// and scoped-token fields. Unknown nested messages are preserved as raw bytes
// in SuccessBytes. An envelope with status 1 must never become a persisted
// session without this payload decoding successfully.
func DecodeSuccess(h18 []byte) (*Session, error) {
	invalid := errors.New("invalid Janus success payload protobuf")

	readBytesField := func(message []byte, want protowire.Number) ([]byte, bool, error) {
		for len(message) > 0 {
			number, wireType, tagSize := protowire.ConsumeTag(message)
			if tagSize < 0 {
				return nil, false, invalid
			}
			message = message[tagSize:]
			if number == want {
				if wireType != protowire.BytesType {
					return nil, false, invalid
				}
				value, size := protowire.ConsumeBytes(message)
				if size < 0 {
					return nil, false, invalid
				}
				return value, true, nil
			}
			size := protowire.ConsumeFieldValue(number, wireType, message)
			if size < 0 {
				return nil, false, invalid
			}
			message = message[size:]
		}
		return nil, false, nil
	}

	b841, found, err := readBytesField(h18, 1)
	if err != nil {
		return nil, err
	}
	if !found || len(b841) == 0 {
		return nil, errors.New("Janus success payload is missing its identity message")
	}

	session := &Session{}
	// Decode h18 byte fields 2, 3, 4, and 11, retaining them in order.
	for _, field := range []protowire.Number{2, 3, 4, 11} {
		if value, ok, err := readBytesField(h18, field); err != nil {
			return nil, err
		} else if ok {
			session.SuccessBytes = append(session.SuccessBytes, append([]byte(nil), value...))
		}
	}

	// Decode b841: string fields 1-5 and ffu0 in field 6.
	for len(b841) > 0 {
		number, wireType, tagSize := protowire.ConsumeTag(b841)
		if tagSize < 0 {
			return nil, invalid
		}
		b841 = b841[tagSize:]
		switch {
		case number >= 1 && number <= 5:
			if wireType != protowire.BytesType {
				return nil, invalid
			}
			value, size := protowire.ConsumeString(b841)
			if size < 0 {
				return nil, invalid
			}
			session.Identity = append(session.Identity, value)
			b841 = b841[size:]
		case number == 6:
			if wireType != protowire.BytesType {
				return nil, invalid
			}
			ffu0, size := protowire.ConsumeBytes(b841)
			if size < 0 {
				return nil, invalid
			}
			if err := decodeFFU0(ffu0, session); err != nil {
				return nil, err
			}
			b841 = b841[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, b841)
			if size < 0 {
				return nil, invalid
			}
			b841 = b841[size:]
		}
	}
	return session, nil
}

// decodeFFU0 extracts the primary string, scoped tokens, and integer field from
// the ffu0 token container.
func decodeFFU0(ffu0 []byte, session *Session) error {
	invalid := errors.New("invalid Janus token container protobuf")
	for len(ffu0) > 0 {
		number, wireType, tagSize := protowire.ConsumeTag(ffu0)
		if tagSize < 0 {
			return invalid
		}
		ffu0 = ffu0[tagSize:]
		switch {
		case number == 1:
			if wireType != protowire.BytesType {
				return invalid
			}
			value, size := protowire.ConsumeString(ffu0)
			if size < 0 {
				return invalid
			}
			session.Primary = value
			ffu0 = ffu0[size:]
		case number == 2:
			if wireType != protowire.BytesType {
				return invalid
			}
			tokenBytes, size := protowire.ConsumeBytes(ffu0)
			if size < 0 {
				return invalid
			}
			token, err := decodeScopedToken(tokenBytes)
			if err != nil {
				return err
			}
			session.ScopedTokens = append(session.ScopedTokens, *token)
			ffu0 = ffu0[size:]
		case number == 3:
			if wireType != protowire.VarintType {
				return invalid
			}
			value, size := protowire.ConsumeVarint(ffu0)
			if size < 0 {
				return invalid
			}
			session.ContainerInt = int64(value)
			ffu0 = ffu0[size:]
		case number == 5:
			if wireType != protowire.BytesType {
				return invalid
			}
			value, size := protowire.ConsumeString(ffu0)
			if size < 0 {
				return invalid
			}
			session.ContainerStr = value
			ffu0 = ffu0[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, ffu0)
			if size < 0 {
				return invalid
			}
			ffu0 = ffu0[size:]
		}
	}
	return nil
}

// decodeScopedToken extracts one ivs0 message: token string (1), scope string
// (2), and the long timing field (3).
func decodeScopedToken(data []byte) (*ScopedToken, error) {
	invalid := errors.New("invalid Janus scoped-token protobuf")
	token := &ScopedToken{}
	for len(data) > 0 {
		number, wireType, tagSize := protowire.ConsumeTag(data)
		if tagSize < 0 {
			return nil, invalid
		}
		data = data[tagSize:]
		switch {
		case number == 1:
			if wireType != protowire.BytesType {
				return nil, invalid
			}
			value, size := protowire.ConsumeString(data)
			if size < 0 {
				return nil, invalid
			}
			token.Token = value
			data = data[size:]
		case number == 2:
			if wireType != protowire.BytesType {
				return nil, invalid
			}
			value, size := protowire.ConsumeString(data)
			if size < 0 {
				return nil, invalid
			}
			token.Scope = value
			data = data[size:]
		case number == 3:
			if wireType != protowire.VarintType {
				return nil, invalid
			}
			value, size := protowire.ConsumeVarint(data)
			if size < 0 {
				return nil, invalid
			}
			token.Expiry = int64(value)
			data = data[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, data)
			if size < 0 {
				return nil, invalid
			}
			data = data[size:]
		}
	}
	return token, nil
}
