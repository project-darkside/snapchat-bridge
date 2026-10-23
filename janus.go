package main

import (
	"errors"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

const janusPasswordLoginPath = "/snapchat.janus.api.LoginService/LoginWithPassword"

type janusIdentifierType protowire.Number

const (
	janusUsername janusIdentifierType = 1
	janusEmail    janusIdentifierType = 2
	janusPhone    janusIdentifierType = 3
)

type janusLoginEnvelope struct {
	Status     uint64
	PayloadTag protowire.Number
	Payload    []byte
}

func parseJanusLoginEnvelope(data []byte) (janusLoginEnvelope, error) {
	var envelope janusLoginEnvelope
	invalid := errors.New("invalid Janus login response protobuf")
	for len(data) > 0 {
		number, wireType, tagSize := protowire.ConsumeTag(data)
		if tagSize < 0 {
			return janusLoginEnvelope{}, invalid
		}
		data = data[tagSize:]
		switch {
		case number == 1:
			if wireType != protowire.VarintType {
				return janusLoginEnvelope{}, invalid
			}
			status, size := protowire.ConsumeVarint(data)
			if size < 0 || status > 1<<31-1 {
				return janusLoginEnvelope{}, invalid
			}
			envelope.Status = status
			data = data[size:]
		case number >= 2 && number <= 13:
			if wireType != protowire.BytesType {
				return janusLoginEnvelope{}, invalid
			}
			payload, size := protowire.ConsumeBytes(data)
			if size < 0 {
				return janusLoginEnvelope{}, invalid
			}
			envelope.PayloadTag = number
			envelope.Payload = append([]byte(nil), payload...)
			data = data[size:]
		default:
			size := protowire.ConsumeFieldValue(number, wireType, data)
			if size < 0 {
				return janusLoginEnvelope{}, invalid
			}
			data = data[size:]
		}
	}
	if envelope.Status == 0 || (envelope.Status == 1 && (envelope.PayloadTag != 2 || len(envelope.Payload) == 0)) {
		return janusLoginEnvelope{}, errors.New("Janus login response is missing its application status or success payload")
	}
	return envelope, nil
}

func marshalJanusPasswordRequest(kind janusIdentifierType, identifier, password string, prerequisites []byte) ([]byte, error) {
	if kind < janusUsername || kind > janusPhone {
		return nil, errors.New("invalid Janus identifier type")
	}
	if identifier == "" || password == "" || !utf8.ValidString(identifier) || !utf8.ValidString(password) {
		return nil, errors.New("Janus identifier and password must be nonempty UTF-8 strings")
	}
	for remaining := prerequisites; len(remaining) > 0; {
		_, _, consumed := protowire.ConsumeField(remaining)
		if consumed < 0 {
			return nil, errors.New("invalid Janus prerequisite protobuf")
		}
		remaining = remaining[consumed:]
	}
	request := protowire.AppendTag(nil, protowire.Number(kind), protowire.BytesType)
	request = protowire.AppendString(request, identifier)
	request = protowire.AppendTag(request, 4, protowire.BytesType)
	request = protowire.AppendString(request, password)
	if len(prerequisites) > 0 {
		request = protowire.AppendTag(request, 15, protowire.BytesType)
		request = protowire.AppendBytes(request, prerequisites)
	}
	return request, nil
}
