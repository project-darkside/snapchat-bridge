package janus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
