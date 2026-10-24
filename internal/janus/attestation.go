package janus

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// AttestationTypeLogin is the recovered LOGIN value for AfxRequest.Type
// (ydx field 3). The APK sets this before invoking the native attestation
// boundary.
const AttestationTypeLogin = 1

// Attestor produces the native attestation blob (the afx output that becomes
// fs60 field 11 / n070 field 15). The wire format of the afx *request* is
// implemented in AfxRequest; how the blob is produced is pluggable so the
// bridge can use either an on-device oracle or, later, a Go reimplementation
// of prh.a.
type Attestor interface {
	// Attest returns the attestation bytes for a serialized afx request.
	Attest(ctx context.Context, request []byte) ([]byte, error)
}

// AfxRequest mirrors ydx, the afx attestation-request message handed to
// yblj.prh.a([B)[B] in the 14.30.0.39 APK. Field names are the obfuscated
// MessageNano names; the wire tags below are the recovered writeTo order.
type AfxRequest struct {
	Path     string       // field 1 (obfuscated "b"): RPC path
	Value    string       // field 2 (obfuscated "c")
	Type     int32        // field 3 (obfuscated "t"): AttestationTypeLogin
	Metadata *AfxMetadata // field 4 (obfuscated "I"): ig3 metadata
	X        []byte       // field 5
	Y        []byte       // field 6
	Z        int64        // field 7
	hasZ     bool         // MessageNano presence bit for field 7
}

// SetZ records field 7 as present, mirroring the ydx.Z presence bit.
func (message *AfxRequest) SetZ(value int64) {
	message.Z = value
	message.hasZ = true
}

// Marshal serializes ydx in writeTo order so the bytes match MessageNano
// output for the same inputs.
func (message *AfxRequest) Marshal() ([]byte, error) {
	if !utf8.ValidString(message.Path) || !utf8.ValidString(message.Value) {
		return nil, errors.New("afx path and value must be valid UTF-8")
	}
	var wire []byte
	if message.Path != "" {
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendString(wire, message.Path)
	}
	if message.Value != "" {
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendString(wire, message.Value)
	}
	if message.Type != 0 {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.Type))
	}
	if message.Metadata != nil {
		encoded, err := message.Metadata.Marshal()
		if err != nil {
			return nil, err
		}
		wire = protowire.AppendTag(wire, 4, protowire.BytesType)
		wire = protowire.AppendBytes(wire, encoded)
	}
	if len(message.X) > 0 {
		wire = protowire.AppendTag(wire, 5, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.X)
	}
	if len(message.Y) > 0 {
		wire = protowire.AppendTag(wire, 6, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Y)
	}
	if message.hasZ {
		wire = protowire.AppendTag(wire, 7, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.Z))
	}
	return wire, nil
}

// AfxMetadata mirrors fg3, the ig3 metadata message nested in AfxRequest field
// 4. Recovered writeTo order; field names are obfuscated.
type AfxMetadata struct {
	Entries []*AfxMetadataEntry // field 1 (obfuscated "b"), repeated
	C       int32               // field 2
	T       int32               // field 3
	I       int64               // field 4
	X       int32               // field 5
}

// Marshal serializes fg3 in writeTo order.
func (message *AfxMetadata) Marshal() ([]byte, error) {
	var wire []byte
	for _, entry := range message.Entries {
		if entry == nil {
			continue
		}
		encoded, err := entry.Marshal()
		if err != nil {
			return nil, err
		}
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendBytes(wire, encoded)
	}
	if message.C != 0 {
		wire = protowire.AppendTag(wire, 2, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.C))
	}
	if message.T != 0 {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.T))
	}
	if message.I != 0 {
		wire = protowire.AppendTag(wire, 4, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.I))
	}
	if message.X != 0 {
		wire = protowire.AppendTag(wire, 5, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.X))
	}
	return wire, nil
}

// AfxMetadataEntry mirrors kte, the repeated element inside fg3 field 1.
type AfxMetadataEntry struct {
	B []string // field 1, repeated string
	C []string // field 2, repeated string
	T int32    // field 3
	I int32    // field 4
}

// Marshal serializes kte in writeTo order.
func (message *AfxMetadataEntry) Marshal() ([]byte, error) {
	var wire []byte
	for _, value := range message.B {
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendString(wire, value)
	}
	for _, value := range message.C {
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendString(wire, value)
	}
	if message.T != 0 {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.T))
	}
	if message.I != 0 {
		wire = protowire.AppendTag(wire, 4, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(message.I))
	}
	return wire, nil
}

// CommandAttestor is an Attestor that shells out to a local command to produce
// the attestation blob. The command receives the serialized afx request on
// stdin and must write the raw attestation bytes to stdout. This is the Option
// A oracle plumbing: point it at an on-device helper (e.g. via `adb shell`) or
// any future local reimplementation of prh.a.
type CommandAttestor struct {
	// Name is the executable (default "adb").
	Name string
	// Args are the executable arguments.
	Args []string
}

// Attest runs the configured command and returns its stdout as the blob.
func (attestor *CommandAttestor) Attest(ctx context.Context, request []byte) ([]byte, error) {
	name := attestor.Name
	if name == "" {
		name = "adb"
	}
	command := exec.CommandContext(ctx, name, attestor.Args...)
	command.Stdin = bytes.NewReader(request)
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}
