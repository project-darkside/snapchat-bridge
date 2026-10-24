package janus

import (
	"context"
	"os"
	"testing"
)

// TestAdbAttestorLiveProbe drives the on-device attestation oracle. It is
// opt-in because it requires an attached non-rooted device with the staged
// helper, installed Snapchat APK, and libscplugin.so already in place. No
// credentials or device identifiers are transmitted.
func TestAdbAttestorLiveProbe(t *testing.T) {
	if os.Getenv("SNAPCHAT_JANUS_ADB_PROBE") != "1" {
		t.Skip("set SNAPCHAT_JANUS_ADB_PROBE=1 to run the on-device attestation oracle probe")
	}

	attestor := DefaultAdbAttestor()
	if adb := os.Getenv("SNAPCHAT_JANUS_ADB"); adb != "" {
		attestor.ADB = adb
	}
	attestor.Serial = os.Getenv("SNAPCHAT_JANUS_ADB_SERIAL")
	attestor.BaseAPK = requireEnv(t, "SNAPCHAT_JANUS_ADB_BASE_APK")
	attestor.OracleDex = requireEnv(t, "SNAPCHAT_JANUS_ADB_ORACLE_DEX")
	attestor.LibPath = requireEnv(t, "SNAPCHAT_JANUS_ADB_LIB")

	request, err := NewLoginAfxRequest(AttestationPathLogin).Marshal()
	if err != nil {
		t.Fatalf("Marshal afx: %v", err)
	}

	blob, err := attestor.Attest(context.Background(), request)
	if err != nil {
		t.Fatalf("Attest: %v", err)
	}
	if len(blob) == 0 {
		t.Fatal("Attest returned an empty blob")
	}
	t.Logf("attestation blob: %d bytes", len(blob))
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("missing required environment variable %s", name)
	}
	return value
}
