package janus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestEndToEndLogin assembles the full recovered n070 password-login request
// and sends it to the official endpoint over Tor. It is opt-in and reads
// disposable test credentials from a local username:password file. The
// password is never logged; session tokens are redacted in the log output.
//
// Required environment:
//
//	SNAPCHAT_JANUS_E2E=1
//	SNAPCHAT_JANUS_CREDS_FILE   username:password file
//	SNAPCHAT_JANUS_ANDROID_ID   16 hex chars (`adb shell settings get secure android_id`)
//	SNAPCHAT_JANUS_ADB          adb executable path
//	SNAPCHAT_JANUS_ADB_BASE_APK    installed com.snapchat.android base.apk path
//	SNAPCHAT_JANUS_ADB_ORACLE_DEX  staged attestation-oracle.zip path
//	SNAPCHAT_JANUS_ADB_LIB         staged libscplugin.so path
//
// Optional: SNAPCHAT_JANUS_PROXY (default 127.0.0.1:9150).
func TestEndToEndLogin(t *testing.T) {
	if os.Getenv("SNAPCHAT_JANUS_E2E") != "1" {
		t.Skip("set SNAPCHAT_JANUS_E2E=1 to run the end-to-end login probe")
	}

	credsFile := os.Getenv("SNAPCHAT_JANUS_CREDS_FILE")
	if credsFile == "" {
		t.Fatal("SNAPCHAT_JANUS_CREDS_FILE is required")
	}
	username, password, err := readCredentials(credsFile)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}

	androidID := os.Getenv("SNAPCHAT_JANUS_ANDROID_ID")
	if androidID == "" {
		t.Fatal("SNAPCHAT_JANUS_ANDROID_ID is required")
	}

	attestation, err := mintAttestation(t)
	if err != nil {
		t.Fatalf("mint attestation: %v", err)
	}
	t.Logf("attestation blob: %d bytes", len(attestation))

	request, err := buildEndToEndRequest(username, password, androidID, attestation)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	encoded, err := request.Marshal()
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	t.Logf("request assembled: %d bytes (password redacted)", len(encoded))

	proxy := os.Getenv("SNAPCHAT_JANUS_PROXY")
	if proxy == "" {
		proxy = "127.0.0.1:9150"
	}
	transport, err := NewTransportWithOptions(
		"Snapchat/14.29.0.50 (JanusProbe; Android 14#0#34; gzip)",
		TransportOptions{ProxyAddr: proxy, SurfaceServerErrors: true},
	)
	if err != nil {
		t.Fatalf("transport: %v", err)
	}
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := transport.LoginWithPassword(ctx, encoded)
	if err != nil {
		t.Fatalf("LoginWithPassword: %v", err)
	}
	t.Logf("raw response: %d bytes", len(response))

	envelope, err := ParseLoginEnvelope(response)
	if err != nil {
		t.Fatalf("parse envelope: %v (raw %s)", err, hex.EncodeToString(response))
	}
	t.Logf("application status: %d (%s)", envelope.Status, envelope.StatusName())
	t.Logf("payload: field %d, %d bytes", envelope.PayloadTag, len(envelope.Payload))
	if envelope.Status != 1 {
		t.Logf("payload hex: %s", hex.EncodeToString(envelope.Payload))
		return
	}

	session, err := DecodeSuccess(envelope.Payload)
	if err != nil {
		t.Fatalf("decode success: %v", err)
	}
	t.Logf("identity: %v", session.Identity)
	t.Logf("container string: %q", session.ContainerStr)
	t.Logf("container int: %d", session.ContainerInt)
	t.Logf("scoped tokens: %d", len(session.ScopedTokens))
	for i, token := range session.ScopedTokens {
		t.Logf("token %d: scope=%q expiry=%d token=%s", i, token.Scope, token.Expiry, redact(token.Token))
	}
}

func readCredentials(path string) (username, password string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	content := strings.TrimRight(string(data), "\r\n")
	if user, pass, ok := strings.Cut(content, ":"); ok {
		return user, pass, nil
	}
	return "", content, nil
}

func mintAttestation(t *testing.T) ([]byte, error) {
	t.Helper()
	attestor := DefaultAdbAttestor()
	if adb := os.Getenv("SNAPCHAT_JANUS_ADB"); adb != "" {
		attestor.ADB = adb
	}
	attestor.Serial = os.Getenv("SNAPCHAT_JANUS_ADB_SERIAL")
	attestor.BaseAPK = os.Getenv("SNAPCHAT_JANUS_ADB_BASE_APK")
	attestor.OracleDex = os.Getenv("SNAPCHAT_JANUS_ADB_ORACLE_DEX")
	attestor.LibPath = os.Getenv("SNAPCHAT_JANUS_ADB_LIB")
	if attestor.BaseAPK == "" || attestor.OracleDex == "" || attestor.LibPath == "" {
		return nil, fmt.Errorf("on-device oracle paths are not configured")
	}
	request, err := NewLoginAfxRequest(AttestationPathLogin).Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal afx: %w", err)
	}
	blob, err := attestor.Attest(context.Background(), request)
	if err != nil {
		return nil, err
	}
	if len(blob) == 0 {
		return nil, fmt.Errorf("oracle returned an empty blob")
	}
	return blob, nil
}

func buildEndToEndRequest(username, password, androidID string, attestation []byte) (*PasswordRequest, error) {
	idBytes, err := hex.DecodeString(androidID)
	if err != nil {
		return nil, fmt.Errorf("android id must be hex: %w", err)
	}
	if len(idBytes) != 8 {
		return nil, fmt.Errorf("android id must decode to 8 bytes, got %d", len(idBytes))
	}
	androidID9 := append([]byte{0x07}, idBytes...)

	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("generate fidelius seed: %w", err)
	}
	key, err := GenerateFideliusKey(seed, 1)
	if err != nil {
		return nil, fmt.Errorf("generate fidelius key: %w", err)
	}
	fidelius, err := NewFideliusInit(nil, androidID9, key.Public, nil, seed, 1)
	if err != nil {
		return nil, fmt.Errorf("fidelius init: %w", err)
	}
	fideliusBytes, err := fidelius.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal fidelius: %w", err)
	}

	cofBytes, err := NewCOFMetadata("", "", nil).Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal cof: %w", err)
	}

	metadata3, err := randomUUID()
	if err != nil {
		return nil, err
	}
	loginAttemptID, err := randomUUID()
	if err != nil {
		return nil, err
	}
	instanceUUID, err := randomUUID()
	if err != nil {
		return nil, err
	}
	prerequisites, err := NewLoginPrerequisites(
		androidID, // fs60 field 1: best-guess device identifier
		"",        // fs60 field 2 (nr60.a / cz60.b)
		"",        // fs60 field 3 (nr60.b / cz60.d)
		metadata3, // fs60 field 5: per-request UUID
		loginAttemptID,
		instanceUUID,
		"", // fs60 field 8: cloud account id (empty on fresh login)
		nil,
		"", // device token id (absent on fresh login)
		attestation,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("prerequisites: %w", err)
	}
	prerequisitesBytes, err := prerequisites.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal prerequisites: %w", err)
	}

	return &PasswordRequest{
		Identifier:     username,
		IdentifierKind: Username,
		Password:       password,
		FideliusInit:   fideliusBytes,
		COFMetadata:    cofBytes,
		Prerequisites:  prerequisitesBytes,
	}, nil
}

func randomUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}

func redact(value string) string {
	if len(value) <= 8 {
		return "(redacted)"
	}
	return value[:4] + "…" + value[len(value)-4:]
}
