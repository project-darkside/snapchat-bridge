package janus

import (
	"encoding/hex"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("bad test hex %q: %v", value, err)
	}
	return decoded
}

func assertWellFormed(t *testing.T, bytes []byte) {
	t.Helper()
	for remaining := bytes; len(remaining) > 0; {
		_, _, consumed := protowire.ConsumeField(remaining)
		if consumed < 0 {
			t.Fatalf("malformed protobuf at offset %d: %x", len(bytes)-len(remaining), remaining)
		}
		remaining = remaining[consumed:]
	}
}

func TestAfxRequestMarshalMinimal(t *testing.T) {
	request := &AfxRequest{Path: "/rpc/x", Type: AttestationTypeLogin}
	got, err := request.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := mustDecodeHex(t, "0a062f7270632f781801")
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("Marshal mismatch:\n got %x\nwant %x", got, want)
	}
	assertWellFormed(t, got)
}

func TestAfxRequestMarshalFields(t *testing.T) {
	request := &AfxRequest{
		Path:  "p",
		Value: "v",
		Type:  2,
		X:     []byte{0xde, 0xad},
		Y:     []byte{0xbe, 0xef},
	}
	request.SetZ(300)
	got, err := request.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := mustDecodeHex(t, "0a017012017618022a02dead3202beef38ac02")
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("Marshal mismatch:\n got %x\nwant %x", got, want)
	}
	assertWellFormed(t, got)
}

func TestAfxRequestMarshalNestedMetadata(t *testing.T) {
	request := &AfxRequest{
		Path: "p",
		Type: AttestationTypeLogin,
		Metadata: &AfxMetadata{
			Entries: []*AfxMetadataEntry{
				{B: []string{"a", "b"}, C: []string{"c"}, T: 5},
			},
			C: 1,
			T: 2,
			I: 3,
			X: 4,
		},
	}
	got, err := request.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := mustDecodeHex(t, "0a0170180122150a0b0a01610a016212016318051001180220032804")
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("Marshal mismatch:\n got %x\nwant %x", got, want)
	}
	assertWellFormed(t, got)
}

func TestAfxRequestMarshalRejectsInvalidUTF8(t *testing.T) {
	request := &AfxRequest{Path: string([]byte{0xff, 0xfe})}
	if _, err := request.Marshal(); err == nil {
		t.Fatal("expected invalid UTF-8 to be rejected")
	}
}
