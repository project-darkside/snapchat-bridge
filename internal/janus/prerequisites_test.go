package janus

import (
	"encoding/hex"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

// TestWgg0Marshal locks in wgg0's repeated int32 field 1 encoding recovered
// from wgg0.writeTo (writeInt32(1, value) per element, unpacked).
func TestWgg0Marshal(t *testing.T) {
	got, err := (&Wgg0{Flags: []int32{1, 2, 3}}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := "080108020803"; hex.EncodeToString(got) != want {
		t.Fatalf("wgg0 mismatch: got %s want %s", hex.EncodeToString(got), want)
	}
}

// TestQ8PMarshal locks in q8p's string field 1 encoding recovered from
// q8p.writeTo (writeString(1, value)).
func TestQ8PMarshal(t *testing.T) {
	token := "tok1"
	got, err := (&Q8P{Token: &token}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := "0a04746f6b31"; hex.EncodeToString(got) != want {
		t.Fatalf("q8p mismatch: got %s want %s", hex.EncodeToString(got), want)
	}
}

// TestLoginPrerequisitesFieldNumbers verifies the fs60 field-number map
// recovered from fs60.writeTo by parsing the marshalled bytes back into
// per-field payloads.
func TestLoginPrerequisitesFieldNumbers(t *testing.T) {
	prereq, err := NewLoginPrerequisites(
		"device", "meta1", "meta2", "meta3", "attempt1", "inst1", "cloud",
		[]int32{1, 2, 3}, "tok1",
		[]byte{0xaa, 0xbb},
		[][]byte{{0x01}, {0x02, 0x03}},
	)
	if err != nil {
		t.Fatalf("NewLoginPrerequisites: %v", err)
	}
	got, err := prereq.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	fields := map[protowire.Number][][]byte{}
	for remaining := got; len(remaining) > 0; {
		number, _, consumed := protowire.ConsumeField(remaining)
		if consumed < 0 {
			t.Fatalf("invalid marshalled fs60: %s", hex.EncodeToString(got))
		}
		fields[number] = append(fields[number], remaining[:consumed])
		remaining = remaining[consumed:]
	}

	assertStringField := func(number protowire.Number, want string) {
		t.Helper()
		entries := fields[number]
		if len(entries) != 1 {
			t.Fatalf("field %d: expected exactly one entry, got %d", number, len(entries))
		}
		_, _, tagLen := protowire.ConsumeTag(entries[0])
		if tagLen < 0 {
			t.Fatalf("field %d: bad tag", number)
		}
		value, n := protowire.ConsumeBytes(entries[0][tagLen:])
		if n < 0 {
			t.Fatalf("field %d: bad string payload", number)
		}
		if string(value) != want {
			t.Fatalf("field %d: got %q want %q", number, string(value), want)
		}
	}

	assertStringField(1, "device")
	assertStringField(2, "meta1")
	assertStringField(3, "meta2")
	assertStringField(4, "attempt1")
	assertStringField(5, "meta3")
	assertStringField(6, "inst1")
	assertStringField(8, "cloud")

	if len(fields[7]) != 1 {
		t.Fatalf("field 7 (wgg0): expected one entry, got %d", len(fields[7]))
	}
	_, _, tagLen := protowire.ConsumeTag(fields[7][0])
	wgg0, err := (&Wgg0{Flags: []int32{1, 2, 3}}).Marshal()
	if err != nil {
		t.Fatalf("wgg0 marshal: %v", err)
	}
	payload, _ := protowire.ConsumeBytes(fields[7][0][tagLen:])
	if hex.EncodeToString(payload) != hex.EncodeToString(wgg0) {
		t.Fatalf("field 7 (wgg0) payload mismatch: got %x want %x", payload, wgg0)
	}

	if len(fields[10]) != 1 {
		t.Fatalf("field 10 (q8p): expected one entry, got %d", len(fields[10]))
	}
	_, _, tagLen = protowire.ConsumeTag(fields[10][0])
	q8p, err := (&Q8P{Token: ptr("tok1")}).Marshal()
	if err != nil {
		t.Fatalf("q8p marshal: %v", err)
	}
	payload, _ = protowire.ConsumeBytes(fields[10][0][tagLen:])
	if hex.EncodeToString(payload) != hex.EncodeToString(q8p) {
		t.Fatalf("field 10 (q8p) payload mismatch: got %x want %x", payload, q8p)
	}

	if len(fields[11]) != 1 {
		t.Fatalf("field 11 (attestation): expected one entry, got %d", len(fields[11]))
	}
	_, _, tagLen = protowire.ConsumeTag(fields[11][0])
	payload, _ = protowire.ConsumeBytes(fields[11][0][tagLen:])
	if hex.EncodeToString(payload) != "aabb" {
		t.Fatalf("field 11 (attestation) payload mismatch: %x", payload)
	}

	if len(fields[14]) != 2 {
		t.Fatalf("field 14 (sequence ids): expected two entries, got %d", len(fields[14]))
	}
}

func ptr(s string) *string { return &s }
