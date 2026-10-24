package janus

import (
	"bytes"
	"encoding/hex"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func int64Ptr(value int64) *int64    { return &value }
func int32Ptr(value int32) *int32    { return &value }
func boolPtr(value bool) *bool       { return &value }
func stringPtr(value string) *string { return &value }

func mustDecode(t *testing.T, value string) []byte {
	t.Helper()
	bytes, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return bytes
}

func TestPasswordRequestMarshalMatchesAPK(t *testing.T) {
	tests := []struct {
		name       string
		kind       IdentifierKind
		identifier string
		want       string
	}{
		{"username", Username, "bridge-test-user", "0a106272696467652d746573742d7573657222136e6f742d612d7265616c2d70617373776f7264"},
		{"email", Email, "bridge-test@example.invalid", "121b6272696467652d74657374406578616d706c652e696e76616c696422136e6f742d612d7265616c2d70617373776f7264"},
		{"phone", Phone, "+15555550100", "1a0c2b313535353535353031303022136e6f742d612d7265616c2d70617373776f7264"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := &PasswordRequest{
				Identifier:     test.identifier,
				IdentifierKind: test.kind,
				Password:       "not-a-real-password",
			}
			got, err := request.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(got) != test.want {
				t.Errorf("request = %x, want APK bytes %s", got, test.want)
			}
		})
	}
}

func TestPasswordRequestMarshalFullFields(t *testing.T) {
	request := &PasswordRequest{
		Identifier:     "u",
		IdentifierKind: Username,
		Password:       "p",
		NumAttempts:    int64Ptr(1),
		FideliusInit:   mustDecode(t, "0a01aa"),
		COFMetadata:    mustDecode(t, "1805"),
		PasswordSource: int32Ptr(2),
		WhatsApp:       boolPtr(true),
		Prerequisites:  mustDecode(t, "5a020102"),
	}
	got, err := request.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "0a017522017028013a030a01aa42021805480258017a045a020102"
	if hex.EncodeToString(got) != want {
		t.Errorf("request = %x, want %s", got, want)
	}
}

func TestPasswordRequestRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		request *PasswordRequest
	}{
		{"identifier type zero", &PasswordRequest{Identifier: "u", Password: "p"}},
		{"identifier type four", &PasswordRequest{Identifier: "u", Password: "p", IdentifierKind: 4}},
		{"empty identifier", &PasswordRequest{Identifier: "", Password: "p", IdentifierKind: Username}},
		{"empty password", &PasswordRequest{Identifier: "u", Password: "", IdentifierKind: Username}},
		{"invalid utf8 identifier", &PasswordRequest{Identifier: "\xff", Password: "p", IdentifierKind: Username}},
		{"invalid fidelius bytes", &PasswordRequest{Identifier: "u", Password: "p", IdentifierKind: Username, FideliusInit: []byte{0x0a}}},
		{"invalid prerequisites", &PasswordRequest{Identifier: "u", Password: "p", IdentifierKind: Username, Prerequisites: []byte{0x5a}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.request.Marshal(); err == nil {
				t.Fatal("invalid request must not marshal")
			}
		})
	}
}

func TestFideliusInitMarshal(t *testing.T) {
	key := &K5U{Derived: []byte{0x01}, C: []byte{0x02}, E: []byte{0x03}, I: int64Ptr(7)}
	message := &FideliusInit{Repeated: [][]byte{{0xaa}}, Key: key, Bytes: []byte{0xbb}}
	got, err := message.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "0a01aa120b0a01011201021a010320071a01bb"
	if hex.EncodeToString(got) != want {
		t.Errorf("fidelius init = %x, want %s", got, want)
	}
}

func TestCOFMetadataMarshal(t *testing.T) {
	message := &COFMetadata{ETag: stringPtr("e"), RoutingTag: stringPtr("r"), Bitmap: int32Ptr(5), Bytes: []byte{0xcc}}
	got, err := message.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "0a016512017218052201cc"
	if hex.EncodeToString(got) != want {
		t.Errorf("cof metadata = %x, want %s", got, want)
	}
}

func TestPrerequisitesMarshal(t *testing.T) {
	message := &Prerequisites{S1: stringPtr("x"), Attestation: []byte{0xde, 0xad}}
	got, err := message.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := "0a01785a02dead"
	if hex.EncodeToString(got) != want {
		t.Errorf("prerequisites = %x, want %s", got, want)
	}
}

func TestDeriveKeyRange(t *testing.T) {
	source := make([]byte, 91)
	for i := range source {
		source[i] = byte(i)
	}
	derived, err := DeriveKeyRange(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(derived) != 65 || !bytes.Equal(derived, source[26:91]) {
		t.Fatalf("derived range = %d bytes, want the 65-byte range [26,91)", len(derived))
	}

	short := source[:30]
	derived, err = DeriveKeyRange(short)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(derived, short[26:]) {
		t.Fatal("short source must copy through its end")
	}

	if _, err := DeriveKeyRange(source[:10]); err == nil {
		t.Fatal("source shorter than 26 bytes must be rejected")
	}
}

func TestParseLoginEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		wire       string
		status     uint64
		payloadTag protowire.Number
		extra      bool
		wantErr    bool
	}{
		{"success envelope", "080112020a00", 1, 2, false, false},
		{"challenge is not success", "08021a00", 2, 3, false, false},
		{"unknown status", "0863", 99, 0, false, false},
		{"field 14 side channel", "080112020a00720101", 1, 2, true, false},
		{"empty", "", 0, 0, false, true},
		{"missing status", "12020a00", 0, 0, false, true},
		{"missing success payload", "0801", 0, 0, false, true},
		{"empty success payload", "08011200", 0, 0, false, true},
		{"conflicting success oneof", "080112020a001a00", 0, 0, false, true},
		{"wrong status type", "0a0101", 0, 0, false, true},
		{"wrong payload type", "08011001", 0, 0, false, true},
		{"truncated status", "0880", 0, 0, false, true},
		{"truncated payload", "080112020a", 0, 0, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, err := ParseLoginEnvelope(mustDecode(t, test.wire))
			if (err != nil) != test.wantErr {
				t.Fatalf("parse error = %v, want error %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if envelope.Status != test.status || envelope.PayloadTag != test.payloadTag {
				t.Fatalf("status/tag = %d/%d, want %d/%d", envelope.Status, envelope.PayloadTag, test.status, test.payloadTag)
			}
			if (len(envelope.Extra) > 0) != test.extra {
				t.Fatalf("extra presence = %v, want %v", len(envelope.Extra) > 0, test.extra)
			}
		})
	}
}

func TestLoginEnvelopeStatusName(t *testing.T) {
	if got := (LoginEnvelope{Status: 1}).StatusName(); got != "success" {
		t.Fatalf("status 1 name = %q, want success", got)
	}
	if got := (LoginEnvelope{Status: 2}).StatusName(); got != "challenge" {
		t.Fatalf("status 2 name = %q, want challenge", got)
	}
	if got := (LoginEnvelope{Status: 17}).StatusName(); got != "integrity_retry" {
		t.Fatalf("status 17 name = %q, want integrity_retry", got)
	}
	if got := (LoginEnvelope{Status: 42}).StatusName(); got != "unknown" {
		t.Fatalf("status 42 name = %q, want unknown", got)
	}
}

func TestDecodeSuccess(t *testing.T) {
	scopedToken := protowire.AppendTag(nil, 1, protowire.BytesType)
	scopedToken = protowire.AppendString(scopedToken, "tok")
	scopedToken = protowire.AppendTag(scopedToken, 2, protowire.BytesType)
	scopedToken = protowire.AppendString(scopedToken, "scope")
	scopedToken = protowire.AppendTag(scopedToken, 3, protowire.VarintType)
	scopedToken = protowire.AppendVarint(scopedToken, 999)

	ffu0 := protowire.AppendTag(nil, 1, protowire.BytesType)
	ffu0 = protowire.AppendString(ffu0, "primary")
	ffu0 = protowire.AppendTag(ffu0, 2, protowire.BytesType)
	ffu0 = protowire.AppendBytes(ffu0, scopedToken)
	ffu0 = protowire.AppendTag(ffu0, 3, protowire.VarintType)
	ffu0 = protowire.AppendVarint(ffu0, 42)
	ffu0 = protowire.AppendTag(ffu0, 5, protowire.BytesType)
	ffu0 = protowire.AppendString(ffu0, "str2")

	b841 := protowire.AppendTag(nil, 1, protowire.BytesType)
	b841 = protowire.AppendString(b841, "identity-one")
	b841 = protowire.AppendTag(b841, 6, protowire.BytesType)
	b841 = protowire.AppendBytes(b841, ffu0)

	h18 := protowire.AppendTag(nil, 1, protowire.BytesType)
	h18 = protowire.AppendBytes(h18, b841)
	h18 = protowire.AppendTag(h18, 2, protowire.BytesType)
	h18 = protowire.AppendBytes(h18, []byte{0xaa})

	session, err := DecodeSuccess(h18)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Identity) != 1 || session.Identity[0] != "identity-one" {
		t.Fatalf("identity = %v", session.Identity)
	}
	if session.Primary != "primary" || session.ContainerInt != 42 || session.ContainerStr != "str2" {
		t.Fatalf("container = %q/%d/%q", session.Primary, session.ContainerInt, session.ContainerStr)
	}
	if len(session.ScopedTokens) != 1 {
		t.Fatalf("scoped tokens = %d", len(session.ScopedTokens))
	}
	token := session.ScopedTokens[0]
	if token.Token != "tok" || token.Scope != "scope" || token.Expiry != 999 {
		t.Fatalf("scoped token = %+v", token)
	}
	if len(session.SuccessBytes) != 1 || !bytes.Equal(session.SuccessBytes[0], []byte{0xaa}) {
		t.Fatalf("success bytes = %x", session.SuccessBytes)
	}
}

func TestDecodeSuccessRejectsMissingIdentity(t *testing.T) {
	h18 := protowire.AppendTag(nil, 2, protowire.BytesType)
	h18 = protowire.AppendBytes(h18, []byte{0xaa})
	if _, err := DecodeSuccess(h18); err == nil {
		t.Fatal("missing identity message must fail")
	}
}
