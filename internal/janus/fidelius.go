package janus

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
)

// FideliusKey holds a freshly generated secp256r1 keypair plus the seed and
// version the login request carries alongside it. The curve is fixed to
// secp256r1, matching lbr.<clinit> and lbr.c() in the installed build.
type FideliusKey struct {
	Private         []byte // PKCS#8 DER (r241.b / r241.d)
	Public          []byte // X.509 SubjectPublicKeyInfo DER, 91 bytes for P-256 (r241.f)
	Point           []byte // 65-byte uncompressed point 0x04 || X || Y (k5u field 1)
	Seed            []byte // master key (r241.e, k5u field 3)
	Version         int64  // version int (r241.i, k5u field 4)
	HashedPublicKey []byte // HMAC-SHA256(master key, SPKI public) (r241.a / r241.c, k5u field 2 and v0u field 1)
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
	hashedPublicKey, err := computeHashedPublicKey(seed, publicDER)
	if err != nil {
		return nil, err
	}
	return &FideliusKey{
		Private:         privateDER,
		Public:          publicDER,
		Point:           point,
		Seed:            append([]byte(nil), seed...),
		Version:         version,
		HashedPublicKey: hashedPublicKey,
	}, nil
}

// FideliusKeyVersion is the key version the installed build targets. r5u.a
// picks 9 or 10 from a config flag, and m5u.e/s3u.k bump every loaded key to 10.
const FideliusKeyVersion = 10

// computeHashedPublicKey reproduces r241.a: HMAC-SHA256 keyed by the master
// key over the SPKI public DER. It is the Fidelius "hashed public key" label.
func computeHashedPublicKey(masterKey, publicDER []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, masterKey)
	if _, err := mac.Write(publicDER); err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil
}

// AndroidIDBytes reproduces n1u.e / lwf case 1: hex-decode the 16-char
// Settings.Secure.ANDROID_ID into 8 bytes and prepend a single 0x07 byte.
func AndroidIDBytes(androidIDHex string) ([]byte, error) {
	decoded, err := hex.DecodeString(androidIDHex)
	if err != nil {
		return nil, errors.New("android id must be hex")
	}
	if len(decoded) != 8 {
		return nil, errors.New("android id must decode to 8 bytes")
	}
	return append([]byte{0x07}, decoded...), nil
}

// NewFideliusInitFromKey assembles v0u from a freshly generated FideliusKey and
// the encoded Android ID, matching vo3.apply case 28: v0u field 1 carries the
// hashed public key as the sole device user id, k5u carries point/hash/master
// key/version, and v0u field 3 carries the 9-byte Android ID.
func NewFideliusInitFromKey(key *FideliusKey, androidID []byte) (*FideliusInit, error) {
	if key == nil {
		return nil, errors.New("fidelius key is nil")
	}
	return &FideliusInit{
		Repeated: [][]byte{key.HashedPublicKey},
		Key: &K5U{
			Derived: key.Point,
			C:       key.HashedPublicKey,
			E:       key.Seed,
			I:       &key.Version,
		},
		Bytes: androidID,
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
