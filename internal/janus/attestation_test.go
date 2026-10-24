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
	request := &AfxRequest{}
	request.SetPath("/rpc/x")
	request.SetType(AttestationTypeLogin)
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
	request := &AfxRequest{}
	request.SetPath("p")
	request.SetValue("v")
	request.SetType(2)
	request.SetX([]byte{0xde, 0xad})
	request.SetY([]byte{0xbe, 0xef})
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
	request := &AfxRequest{}
	request.SetPath("p")
	request.SetType(AttestationTypeLogin)
	metadata := &AfxMetadata{}
	entry := &AfxMetadataEntry{B: []string{"a", "b"}, C: []string{"c"}}
	entry.SetT(5)
	metadata.Entries = []*AfxMetadataEntry{entry}
	metadata.SetC(true)
	metadata.SetT(true)
	metadata.SetI(3)
	metadata.SetX(false)
	request.Metadata = metadata
	got, err := request.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// 0a0170       field 1 = "p"
	// 1801         field 3 = 1
	// 2215 ...     field 4 = 21-byte fg3:
	//   0a0b ...                     field 1 = 11-byte kte (see below)
	//     0a0161 0a0162 120163 1805  kte: b=["a","b"] c=["c"] t=5
	//   1001                         fg3 field 2 = true
	//   1801                         fg3 field 3 = true
	//   2003                         fg3 field 4 = 3
	//   2800                         fg3 field 5 = false (presence bit set)
	want := mustDecodeHex(t, "0a0170180122150a0b0a01610a016212016318051001180120032800")
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("Marshal mismatch:\n got %x\nwant %x", got, want)
	}
	assertWellFormed(t, got)
}

func TestAfxRequestMarshalRejectsInvalidUTF8(t *testing.T) {
	request := &AfxRequest{}
	request.SetPath(string([]byte{0xff, 0xfe}))
	if _, err := request.Marshal(); err == nil {
		t.Fatal("expected invalid UTF-8 to be rejected")
	}
}

func TestNewLoginAfxRequest(t *testing.T) {
	request := NewLoginAfxRequest(AttestationPathLogin)
	got, err := request.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	assertWellFormed(t, got)

	fields := decodeMessageFields(t, got)
	if len(fields[1]) != 0 {
		t.Fatalf("field 1 (path) must be absent in the LOGIN afx, got %x", fields[1])
	}
	if len(fields[2]) != 1 || string(fields[2][0]) != AttestationPathLogin {
		t.Fatalf("field 2 must be the login path, got %q", fields[2])
	}
	if len(fields[3]) != 1 || decodeVarint(t, fields[3][0]) != AttestationTypeLogin {
		t.Fatalf("field 3 must be LOGIN type 1, got %x", fields[3])
	}
	if len(fields[4]) != 1 {
		t.Fatal("field 4 (fg3 metadata) must be present")
	}
	if len(fields[5]) != 1 || len(fields[5][0]) != 0 {
		t.Fatalf("field 5 must be present and empty, got %x", fields[5])
	}
	if len(fields[6]) != 0 {
		t.Fatal("field 6 must be absent in the LOGIN afx")
	}
	if len(fields[7]) != 0 {
		t.Fatal("field 7 must be absent in the LOGIN afx")
	}

	metadataFields := decodeMessageFields(t, fields[4][0])
	if len(metadataFields[1]) != len(attestationMetadataEntries) {
		t.Fatalf("expected %d kte entries, got %d", len(attestationMetadataEntries), len(metadataFields[1]))
	}
	for index, entry := range metadataFields[1] {
		gotFields := decodeMessageFields(t, entry)
		wantEntry := attestationMetadataEntries[index]
		assertStringValues(t, gotFields[1], wantEntry.B)
		assertStringValues(t, gotFields[2], wantEntry.C)
		switch index {
		case 0:
			// entry 0 has t=5 and I=true.
			assertVarint(t, gotFields[3][0], 5)
			assertVarint(t, gotFields[4][0], 1)
		case 1:
			// entry 1 has t=1 and I=false (presence bit still set, value 0).
			assertVarint(t, gotFields[3][0], 1)
			assertVarint(t, gotFields[4][0], 0)
		case 2:
			assertVarint(t, gotFields[3][0], 5)
			assertVarint(t, gotFields[4][0], 1)
		}
	}
}

// decodeMessageFields parses a protobuf message into a map of field number to
// every occurrence of that field. Bytes-type values are the decoded contents;
// varint values are the raw varint bytes.
func decodeMessageFields(t *testing.T, bytes []byte) map[int][][]byte {
	t.Helper()
	fields := make(map[int][][]byte)
	for remaining := bytes; len(remaining) > 0; {
		number, typ, tagConsumed := protowire.ConsumeTag(remaining)
		if tagConsumed < 0 {
			t.Fatalf("malformed tag in %x", remaining)
		}
		remaining = remaining[tagConsumed:]
		switch typ {
		case protowire.BytesType:
			value, consumed := protowire.ConsumeBytes(remaining)
			if consumed < 0 {
				t.Fatalf("malformed field %d in %x", number, remaining)
			}
			fields[int(number)] = append(fields[int(number)], value)
			remaining = remaining[consumed:]
		case protowire.VarintType:
			_, consumed := protowire.ConsumeVarint(remaining)
			if consumed < 0 {
				t.Fatalf("malformed field %d in %x", number, remaining)
			}
			fields[int(number)] = append(fields[int(number)], remaining[:consumed])
			remaining = remaining[consumed:]
		default:
			t.Fatalf("unexpected field type %d in %x", typ, remaining)
		}
	}
	return fields
}

func assertStringValues(t *testing.T, got [][]byte, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("string list length mismatch: got %d want %d", len(got), len(want))
	}
	for index, value := range want {
		if string(got[index]) != value {
			t.Fatalf("string list item %d mismatch: got %q want %q", index, got[index], value)
		}
	}
}

func decodeVarint(t *testing.T, raw []byte) uint64 {
	t.Helper()
	value, consumed := protowire.ConsumeVarint(raw)
	if consumed < 0 || consumed != len(raw) {
		t.Fatalf("malformed varint field %x", raw)
	}
	return value
}

func assertVarint(t *testing.T, got []byte, want uint64) {
	t.Helper()
	if decodeVarint(t, got) != want {
		t.Fatalf("varint mismatch: got %d want %d", decodeVarint(t, got), want)
	}
}
