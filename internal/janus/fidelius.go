package janus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
)

// FideliusKey holds a freshly generated secp256r1 keypair plus the seed and
// version the login request carries alongside it. The curve is fixed to
// secp256r1, matching lbr.<clinit> and lbr.c() in the installed build.
type FideliusKey struct {
	Private []byte // PKCS#8 DER (r241.b)
	Public  []byte // X.509 SubjectPublicKeyInfo DER, 91 bytes for P-256 (r241.f)
	Point   []byte // 65-byte uncompressed point 0x04 || X || Y (k5u field 1)
	Seed    []byte // seed bytes (r241.e, k5u field 3)
	Version int64  // version int (r241.i, k5u field 4)
}

// GenerateFideliusKey creates a fresh secp256r1 keypair and extracts the
// uncompressed public point exactly as the app does: X.509 DER bytes [26:91).
func GenerateFideliusKey(seed []byte, version int64) (*FideliusKey, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		return nil, err
	}
	point, err := ExtractPoint(publicDER)
	if err != nil {
		return nil, err
	}
	return &FideliusKey{
		Private: privateDER,
		Public:  publicDER,
		Point:   point,
		Seed:    append([]byte(nil), seed...),
		Version: version,
	}, nil
}

// ExtractPoint reproduces mor0.d / uh3.w1(26, 91): the 65-byte uncompressed EC
// point carried in bytes [26:91) of a P-256 X.509 SubjectPublicKeyInfo DER.
func ExtractPoint(publicDER []byte) ([]byte, error) {
	if len(publicDER) != 91 {
		return nil, errors.New("Fidelius public key must be 91-byte P-256 X.509 DER")
	}
	if publicDER[25] != 0x00 || publicDER[26] != 0x04 {
		return nil, errors.New("Fidelius public key has unexpected DER layout")
	}
	return append([]byte(nil), publicDER[26:91]...), nil
}
