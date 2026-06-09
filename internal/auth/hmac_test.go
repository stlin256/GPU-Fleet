package auth

import (
	"net/http"
	"testing"
	"time"
)

func TestSignatureBindsDeviceID(t *testing.T) {
	body := []byte(`{"ok":true}`)
	at := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	nonce := "nonce-for-device-binding"
	secret := "shared-secret"
	path := "/api/v1/agent/heartbeat"

	deviceA := Sign(http.MethodPost, path, body, "device-a", secret, at, nonce)
	deviceB := Sign(http.MethodPost, path, body, "device-b", secret, at, nonce)
	if deviceA == deviceB {
		t.Fatal("signature must change when device id changes")
	}
	if err := Verify(http.MethodPost, path, body, "device-a", at.Format(time.RFC3339), nonce, deviceA, secret, at, time.Minute); err != nil {
		t.Fatalf("expected signature to verify for original device: %v", err)
	}
	if err := Verify(http.MethodPost, path, body, "device-b", at.Format(time.RFC3339), nonce, deviceA, secret, at, time.Minute); err == nil {
		t.Fatal("expected signature for device-a to fail for device-b")
	}
}

func TestLegacySignatureVerifiesOnlyWithLegacyVerifier(t *testing.T) {
	body := []byte(`{"ok":true}`)
	at := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	nonce := "nonce-for-legacy-agent"
	secret := "shared-secret"
	path := "/api/v1/agent/heartbeat"

	legacy := LegacySign(http.MethodPost, path, body, secret, at, nonce)
	if err := VerifyLegacy(http.MethodPost, path, body, at.Format(time.RFC3339), nonce, legacy, secret, at, time.Minute); err != nil {
		t.Fatalf("expected legacy signature to verify with legacy verifier: %v", err)
	}
	if err := Verify(http.MethodPost, path, body, "device-a", at.Format(time.RFC3339), nonce, legacy, secret, at, time.Minute); err == nil {
		t.Fatal("expected legacy signature to fail the device-bound verifier")
	}
}
