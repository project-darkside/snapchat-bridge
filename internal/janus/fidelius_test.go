package janus

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"math/big"
	"testing"
)

func TestExtractPoint(t *testing.T) {
	// A real P-256 key produced by the Go crypto stack.
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(publicDER) != 91 {
		t.Fatalf("P-256 X.509 SPKI length = %d, want 91", len(publicDER))
	}
	point, err := ExtractPoint(publicDER)
	if err != nil {
		t.Fatal(err)
	}
	if len(point) != 65 || point[0] != 0x04 {
		t.Fatalf("extracted point is %d bytes with prefix 0x%02x, want 65 bytes with 0x04", len(point), point[0])
	}

	// The extracted point must unmarshal back to the original public key.
	x, y := elliptic.Unmarshal(elliptic.P256(), point)
	if x == nil || x.Cmp(private.PublicKey.X) != 0 || y.Cmp(private.PublicKey.Y) != 0 {
		t.Fatal("extracted point does not round-trip to the original public key")
	}
}

func TestExtractPointRejectsBadDER(t *testing.T) {
	if _, err := ExtractPoint(nil); err == nil {
		t.Fatal("empty DER must be rejected")
	}
	if _, err := ExtractPoint(make([]byte, 91)); err == nil {
		t.Fatal("DER with a non-point layout must be rejected")
	}
}

func TestGenerateFideliusKey(t *testing.T) {
	seed := []byte("seed-bytes")
	key, err := GenerateFideliusKey(seed, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(key.Public) != 91 || len(key.Point) != 65 {
		t.Fatalf("public/point lengths = %d/%d, want 91/65", len(key.Public), len(key.Point))
	}
	if string(key.Seed) != string(seed) || key.Version != 10 {
		t.Fatalf("seed/version = %q/%d", key.Seed, key.Version)
	}

	// The private DER must parse back to a P-256 key whose public point
	// matches the extracted point.
	parsed, err := x509.ParsePKCS8PrivateKey(key.Private)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || ecKey.Curve != elliptic.P256() {
		t.Fatal("private key is not a P-256 ECDSA key")
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), key.Point)
	if x.Cmp(ecKey.PublicKey.X) != 0 || y.Cmp(ecKey.PublicKey.Y) != 0 {
		t.Fatal("key point does not match the generated private key")
	}

	// Distinct keys must produce distinct points.
	key2, err := GenerateFideliusKey(seed, 10)
	if err != nil {
		t.Fatal(err)
	}
	if big.NewInt(0).SetBytes(key.Point).Cmp(big.NewInt(0).SetBytes(key2.Point)) == 0 {
		t.Fatal("two generated keys share the same public point")
	}
}

// TestHashedPublicKey locks in r241.a: HMAC-SHA256(master key, SPKI public).
func TestHashedPublicKey(t *testing.T) {
	master := []byte("master-key-32-bytes-xxxxxxxxxxxxxx")
	public := make([]byte, 91)
	mac := hmac.New(sha256.New, master)
	if _, err := mac.Write(public); err != nil {
		t.Fatal(err)
	}
	want := mac.Sum(nil)

	got, err := computeHashedPublicKey(master, public)
	if err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(got, want) {
		t.Fatalf("hashed public key mismatch: got %x want %x", got, want)
	}

	key, err := GenerateFideliusKey([]byte("seed"), 10)
	if err != nil {
		t.Fatal(err)
	}
	mac = hmac.New(sha256.New, key.Seed)
	mac.Write(key.Public)
	if !hmac.Equal(key.HashedPublicKey, mac.Sum(nil)) {
		t.Fatal("GenerateFideliusKey did not derive HashedPublicKey from master key + SPKI public")
	}
}

// TestAndroidIDBytes locks in the 0x07-prefixed 9-byte Android ID encoding.
func TestAndroidIDBytes(t *testing.T) {
	got, err := AndroidIDBytes("fe2b3b401aa9dc4e")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x07, 0xfe, 0x2b, 0x3b, 0x40, 0x1a, 0xa9, 0xdc, 0x4e}
	if !bytes.Equal(got, want) {
		t.Fatalf("android id bytes = %x, want %x", got, want)
	}
	for _, bad := range []string{"", "abc", "fe2b3b401aa9dc4e00"} {
		if _, err := AndroidIDBytes(bad); err == nil {
			t.Fatalf("AndroidIDBytes(%q) must fail", bad)
		}
	}
}

// TestNewFideliusInitFromKey locks in the vo3.apply case 28 v0u assembly.
func TestNewFideliusInitFromKey(t *testing.T) {
	key, err := GenerateFideliusKey(make([]byte, 32), FideliusKeyVersion)
	if err != nil {
		t.Fatal(err)
	}
	androidID := []byte{0x07, 1, 2, 3, 4, 5, 6, 7, 8}
	init, err := NewFideliusInitFromKey(key, androidID)
	if err != nil {
		t.Fatal(err)
	}
	if len(init.Repeated) != 1 || !bytes.Equal(init.Repeated[0], key.HashedPublicKey) {
		t.Fatalf("v0u field 1 must carry the hashed public key as the sole device user id")
	}
	if init.Key == nil {
		t.Fatal("v0u field 2 (k5u) is nil")
	}
	if !bytes.Equal(init.Key.Derived, key.Point) {
		t.Fatal("k5u field 1 must be the extracted public point")
	}
	if !bytes.Equal(init.Key.C, key.HashedPublicKey) {
		t.Fatal("k5u field 2 must be the hashed public key")
	}
	if !bytes.Equal(init.Key.E, key.Seed) {
		t.Fatal("k5u field 3 must be the master key")
	}
	if init.Key.I == nil || *init.Key.I != FideliusKeyVersion {
		t.Fatalf("k5u field 4 must be version %d", FideliusKeyVersion)
	}
	if !bytes.Equal(init.Bytes, androidID) {
		t.Fatal("v0u field 3 must carry the 9-byte Android ID")
	}
}
