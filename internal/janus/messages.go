// Package janus implements the Snapchat Janus password-login wire protocol
// recovered from the installed Android 14.29.0.50 build. It is a diagnostic
// prototype, not a complete login implementation: device attestation, Fidelius
// key generation, and server acceptance of the assembled request remain
// unverified. See docs/development/PASSWORD_LOGIN_FLOW.md for provenance.
package janus

import (
	"errors"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// LoginWithPasswordPath is the official password-login RPC method.
const LoginWithPasswordPath = "/snapchat.janus.api.LoginService/LoginWithPassword"

// IdentifierKind selects the n070 identifier oneof (fields 1, 2, 3).
type IdentifierKind int

const (
	Username IdentifierKind = 1
	Email    IdentifierKind = 2
	Phone    IdentifierKind = 3
)

// Valid reports whether kind is one of the recovered identifier oneof values.
func (kind IdentifierKind) Valid() bool { return kind >= Username && kind <= Phone }

// PasswordRequest mirrors the recovered n070 message. Fields that the bridge
// cannot generate from credentials alone (FideliusInit, COFMetadata, and
// Prerequisites) are supplied as pre-encoded protobuf bytes, which the
// marshaller embeds verbatim as their nested messages.
type PasswordRequest struct {
	Identifier     string
	IdentifierKind IdentifierKind
	Password       string

	NumAttempts    *int64 // field 5
	FideliusInit   []byte // field 7 (pre-encoded v0u)
	COFMetadata    []byte // field 8 (pre-encoded c3e)
	PasswordSource *int32 // field 9
	WhatsApp       *bool  // field 11
	Prerequisites  []byte // field 15 (pre-encoded fs60)
}

// Marshal serializes the request in the APK's writeTo field order so the bytes
// match MessageNano output for the same inputs.
func (request *PasswordRequest) Marshal() ([]byte, error) {
	if !request.IdentifierKind.Valid() {
		return nil, errors.New("invalid Janus identifier type")
	}
	if request.Identifier == "" || request.Password == "" {
		return nil, errors.New("Janus identifier and password must be non-empty")
	}
	if !utf8.ValidString(request.Identifier) || !utf8.ValidString(request.Password) {
		return nil, errors.New("Janus identifier and password must be valid UTF-8")
	}
	if err := validateNested(request.FideliusInit, "Fidelius init"); err != nil {
		return nil, err
	}
	if err := validateNested(request.COFMetadata, "COF metadata"); err != nil {
		return nil, err
	}
	if err := validateNested(request.Prerequisites, "prerequisites"); err != nil {
		return nil, err
	}

	message := protowire.AppendTag(nil, protowire.Number(request.IdentifierKind), protowire.BytesType)
	message = protowire.AppendString(message, request.Identifier)
	message = protowire.AppendTag(message, 4, protowire.BytesType)
	message = protowire.AppendString(message, request.Password)
	if request.NumAttempts != nil {
		message = protowire.AppendTag(message, 5, protowire.VarintType)
		message = protowire.AppendVarint(message, uint64(*request.NumAttempts))
	}
	if len(request.FideliusInit) > 0 {
		message = protowire.AppendTag(message, 7, protowire.BytesType)
		message = protowire.AppendBytes(message, request.FideliusInit)
	}
	if len(request.COFMetadata) > 0 {
		message = protowire.AppendTag(message, 8, protowire.BytesType)
		message = protowire.AppendBytes(message, request.COFMetadata)
	}
	if request.PasswordSource != nil {
		message = protowire.AppendTag(message, 9, protowire.VarintType)
		message = protowire.AppendVarint(message, uint64(*request.PasswordSource))
	}
	if request.WhatsApp != nil {
		message = protowire.AppendTag(message, 11, protowire.VarintType)
		if *request.WhatsApp {
			message = protowire.AppendVarint(message, 1)
		} else {
			message = protowire.AppendVarint(message, 0)
		}
	}
	if len(request.Prerequisites) > 0 {
		message = protowire.AppendTag(message, 15, protowire.BytesType)
		message = protowire.AppendBytes(message, request.Prerequisites)
	}
	return message, nil
}

// validateNested confirms nested message bytes are a well-formed protobuf
// before they are embedded, matching the bridge's fail-closed parsing policy.
func validateNested(bytes []byte, name string) error {
	for remaining := bytes; len(remaining) > 0; {
		_, _, consumed := protowire.ConsumeField(remaining)
		if consumed < 0 {
			return errors.New("invalid Janus " + name + " protobuf")
		}
		remaining = remaining[consumed:]
	}
	return nil
}

// FideliusInit mirrors v0u, the Fidelius initialization message sent as
// password-request field 7. The accompanying r241 user-key context is kept
// locally and is not part of the wire message.
type FideliusInit struct {
	Repeated [][]byte // field 1, repeated bytes (n1u.h(), base64-decoded)
	Key      *K5U     // field 2
	Bytes    []byte   // field 3 (n1u.e())
}

// Marshal serializes v0u in writeTo order.
func (message *FideliusInit) Marshal() ([]byte, error) {
	var wire []byte
	for _, value := range message.Repeated {
		if value == nil {
			continue
		}
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendBytes(wire, value)
	}
	if message.Key != nil {
		encoded, err := message.Key.Marshal()
		if err != nil {
			return nil, err
		}
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendBytes(wire, encoded)
	}
	if len(message.Bytes) > 0 {
		wire = protowire.AppendTag(wire, 3, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Bytes)
	}
	return wire, nil
}

// NewFideliusInit assembles v0u from recovered user-key context (r241) and the
// n1u inputs. f is r241.f() (stored bytes); the APK sends f[26:91) as k5u
// field 1 via the mor0.d/uh3.w1 transform. c and e are r241.c()/r241.e();
// i is r241.i().
func NewFideliusInit(n1uH [][]byte, n1uE []byte, f, c, e []byte, i int64) (*FideliusInit, error) {
	derived, err := DeriveKeyRange(f)
	if err != nil {
		return nil, err
	}
	key := &K5U{Derived: derived, C: c, E: e, I: &i}
	return &FideliusInit{Repeated: n1uH, Key: key, Bytes: n1uE}, nil
}

// DeriveKeyRange reproduces mor0.d/uh3.w1: Arrays.copyOfRange(bytes, 26, 91).
// A source shorter than 26 bytes is rejected; a source of at least 91 bytes
// yields exactly 65 bytes.
func DeriveKeyRange(bytes []byte) ([]byte, error) {
	if len(bytes) < 26 {
		return nil, errors.New("Fidelius key source is shorter than the 26-byte transform start")
	}
	end := 91
	if len(bytes) < end {
		end = len(bytes)
	}
	return append([]byte(nil), bytes[26:end]...), nil
}

// K5U mirrors k5u, the Fidelius key message inside v0u.
type K5U struct {
	Derived []byte // field 1, r241.f()[26:91)
	C       []byte // field 2, r241.c()
	E       []byte // field 3, r241.e()
	I       *int64 // field 4, r241.i()
}

// Marshal serializes k5u in writeTo order.
func (message *K5U) Marshal() ([]byte, error) {
	var wire []byte
	if len(message.Derived) > 0 {
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Derived)
	}
	if len(message.C) > 0 {
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.C)
	}
	if len(message.E) > 0 {
		wire = protowire.AppendTag(wire, 3, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.E)
	}
	if message.I != nil {
		wire = protowire.AppendTag(wire, 4, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(*message.I))
	}
	return wire, nil
}

// COFMetadata mirrors c3e, the client configuration-sync metadata sent as
// password-request field 8. It is not a device credential.
//
// Recovered from xq60.a, which maps mr60 (LoginConfigs(cofSyncMetadata,
// cofRoutingTag)) into c3e: field 1 = cofRoutingTag (mr60.a), field 2 =
// cofSyncMetadata.etag (b3e.a), field 3 = the hardcoded 385 set by c3e.d(),
// and field 4 = cofSyncMetadata.bitmap (b3e.b, a byte array).
type COFMetadata struct {
	RoutingTag *string // field 1 (cofRoutingTag)
	ETag       *string // field 2 (cofSyncMetadata.etag)
	Version    *int32  // field 3, always 385
	Bitmap     []byte  // field 4 (cofSyncMetadata.bitmap)
}

// COFMetadataVersion is the value c3e.d() writes to field 3. It is a fixed
// constant in the APK, not a runtime bitmap.
const COFMetadataVersion = 385

// NewCOFMetadata assembles c3e exactly as xq60.a does: routing tag into field
// 1, etag into field 2, the fixed 385 into field 3, and the bitmap into field
// 4. Empty etag/routing-tag strings still set their presence bits, matching
// the MessageNano setters.
func NewCOFMetadata(routingTag, etag string, bitmap []byte) *COFMetadata {
	version := int32(COFMetadataVersion)
	return &COFMetadata{
		RoutingTag: &routingTag,
		ETag:       &etag,
		Version:    &version,
		Bitmap:     bitmap,
	}
}

// Marshal serializes c3e in writeTo order.
func (message *COFMetadata) Marshal() ([]byte, error) {
	var wire []byte
	if message.RoutingTag != nil {
		wire = protowire.AppendTag(wire, 1, protowire.BytesType)
		wire = protowire.AppendString(wire, *message.RoutingTag)
	}
	if message.ETag != nil {
		wire = protowire.AppendTag(wire, 2, protowire.BytesType)
		wire = protowire.AppendString(wire, *message.ETag)
	}
	if message.Version != nil {
		wire = protowire.AppendTag(wire, 3, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(*message.Version))
	}
	if len(message.Bitmap) > 0 {
		wire = protowire.AppendTag(wire, 4, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Bitmap)
	}
	return wire, nil
}

// Prerequisites mirrors fs60, the request prerequisite message sent as
// password-request field 15. Field numbers and semantics were re-verified
// against fs60.writeTo in the installed 14.29.0.50 build:
//
//	field 1  = device/app identifier (rod.b())
//	field 2  = request metadata string 1 (nr60.a)
//	field 3  = request metadata string 2 (nr60.b)
//	field 4  = login attempt id (nr60.d)
//	field 5  = request metadata string 3
//	field 6  = instance uuid (su60.a)
//	field 7  = wgg0: COF sequence/feature flags (su60.c), repeated int32
//	field 8  = cloud account id (su60.e)
//	field 10 = q8p: device token id (rxf0.j()), omitted when empty
//	field 11 = client attestation blob (su60.d)
//	field 14 = COF sequence ids (su60.b), repeated serialized messages
//
// Fields 7 and 10 are nested messages; encode them with Wgg0 and Q8P or use
// NewLoginPrerequisites.
type Prerequisites struct {
	S1, S2, S3, S4, S5, S6 *string  // fields 1-6
	Field7                 []byte   // field 7 (wgg0), pre-encoded
	S8                     *string  // field 8
	Field10                []byte   // field 10 (q8p), pre-encoded
	Attestation            []byte   // field 11, native attestation output
	S12, S13               *string  // fields 12-13
	Repeated14             [][]byte // field 14, repeated bytes
}

// Marshal serializes fs60 in writeTo order.
func (message *Prerequisites) Marshal() ([]byte, error) {
	writeString := func(wire []byte, number protowire.Number, value *string) []byte {
		if value == nil {
			return wire
		}
		wire = protowire.AppendTag(wire, number, protowire.BytesType)
		return protowire.AppendString(wire, *value)
	}
	var wire []byte
	wire = writeString(wire, 1, message.S1)
	wire = writeString(wire, 2, message.S2)
	wire = writeString(wire, 3, message.S3)
	wire = writeString(wire, 4, message.S4)
	wire = writeString(wire, 5, message.S5)
	wire = writeString(wire, 6, message.S6)
	if len(message.Field7) > 0 {
		wire = protowire.AppendTag(wire, 7, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Field7)
	}
	wire = writeString(wire, 8, message.S8)
	if len(message.Field10) > 0 {
		wire = protowire.AppendTag(wire, 10, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Field10)
	}
	if len(message.Attestation) > 0 {
		wire = protowire.AppendTag(wire, 11, protowire.BytesType)
		wire = protowire.AppendBytes(wire, message.Attestation)
	}
	wire = writeString(wire, 12, message.S12)
	wire = writeString(wire, 13, message.S13)
	for _, value := range message.Repeated14 {
		if value == nil {
			continue
		}
		wire = protowire.AppendTag(wire, 14, protowire.BytesType)
		wire = protowire.AppendBytes(wire, value)
	}
	return wire, nil
}

// Wgg0 mirrors wgg0, the fs60 field 7 message carrying the COF
// sequence/feature flags. writeTo emits each element unpacked as
// writeInt32(1, value): repeated int32 field 1.
type Wgg0 struct {
	Flags []int32 // field 1, repeated int32
}

// Marshal serializes wgg0 in writeTo order.
func (message *Wgg0) Marshal() ([]byte, error) {
	var wire []byte
	for _, flag := range message.Flags {
		wire = protowire.AppendTag(wire, 1, protowire.VarintType)
		wire = protowire.AppendVarint(wire, uint64(flag))
	}
	return wire, nil
}

// Q8P mirrors q8p, the fs60 field 10 message carrying the device token id.
// writeTo emits writeString(1, value): string field 1.
type Q8P struct {
	Token *string // field 1
}

// Marshal serializes q8p in writeTo order. A nil token yields an empty
// message, matching a q8p whose presence bit was never set.
func (message *Q8P) Marshal() ([]byte, error) {
	if message.Token == nil {
		return []byte{}, nil
	}
	wire := protowire.AppendTag(nil, 1, protowire.BytesType)
	return protowire.AppendString(wire, *message.Token), nil
}

// NewLoginPrerequisites assembles fs60 from the recovered login value sources.
// deviceID is rod.b(); metadata1/2/3 are the three request metadata strings
// (nr60.a, nr60.b, and the third string); loginAttemptID is nr60.d;
// instanceUUID is su60.a; flags are the COF sequence/feature flags (wgg0 field
// 1, omitted when empty); cloudAccountID is su60.e; deviceTokenID is rxf0.j()
// (q8p field 1, omitted when empty); attestation is su60.d; sequenceIDs are
// su60.b (serialized COF sequence messages, omitted when empty).
func NewLoginPrerequisites(deviceID, metadata1, metadata2, metadata3, loginAttemptID, instanceUUID, cloudAccountID string, flags []int32, deviceTokenID string, attestation []byte, sequenceIDs [][]byte) (*Prerequisites, error) {
	prereq := &Prerequisites{
		S1:          &deviceID,
		S2:          &metadata1,
		S3:          &metadata2,
		S4:          &loginAttemptID,
		S5:          &metadata3,
		S6:          &instanceUUID,
		S8:          &cloudAccountID,
		Attestation: attestation,
		Repeated14:  sequenceIDs,
	}
	if len(flags) > 0 {
		encoded, err := (&Wgg0{Flags: flags}).Marshal()
		if err != nil {
			return nil, err
		}
		prereq.Field7 = encoded
	}
	if deviceTokenID != "" {
		token := deviceTokenID
		encoded, err := (&Q8P{Token: &token}).Marshal()
		if err != nil {
			return nil, err
		}
		prereq.Field10 = encoded
	}
	return prereq, nil
}
