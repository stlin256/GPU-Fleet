package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gpufleet/internal/model"
)

func TestAgentUpdaterAppliesSignedArtifact(t *testing.T) {
	artifactBytes := []byte("new-agent-binary")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var manifestURL string
	var artifactURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/artifact":
			_, _ = w.Write(artifactBytes)
		case "/manifest":
			manifest := signedManifest(t, privateKey, UpdateManifest{
				Version: "0.1.10",
				Artifacts: []UpdateArtifact{{
					OS:        "linux",
					Arch:      "amd64",
					URL:       artifactURL,
					SHA256:    sha256Hex(artifactBytes),
					SizeBytes: int64(len(artifactBytes)),
				}},
			})
			_ = jsonResponse(w, manifest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manifestURL = server.URL + "/manifest"
	artifactURL = server.URL + "/artifact"

	root := t.TempDir()
	exe := filepath.Join(root, "gpufleet-agent")
	if err := os.WriteFile(exe, []byte("old-agent-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	updater := AgentUpdater{
		CurrentVersion: "0.1.9",
		OS:             "linux",
		Arch:           "amd64",
		ExecutablePath: exe,
		StagingDir:     root,
	}
	result, err := updater.ApplyPolicy(context.Background(), model.AgentUpdatePolicy{
		Enabled:        true,
		Mode:           "patch",
		DesiredVersion: "0.1.10",
		ManifestURL:    manifestURL,
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "applied" || result.TargetVersion != "0.1.10" {
		t.Fatalf("unexpected update result: %+v", result)
	}
	if got, err := os.ReadFile(exe); err != nil || string(got) != string(artifactBytes) {
		t.Fatalf("expected executable replacement, got %q err=%v", string(got), err)
	}
	if got, err := os.ReadFile(exe + ".bak"); err != nil || string(got) != "old-agent-binary" {
		t.Fatalf("expected backup of old executable, got %q err=%v", string(got), err)
	}
}

func TestAgentUpdaterRejectsBadSignatureAndSHA(t *testing.T) {
	artifactBytes := []byte("new-agent-binary")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var artifactURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/artifact":
			_, _ = w.Write(artifactBytes)
		case "/bad-signature":
			manifest := signedManifest(t, privateKey, UpdateManifest{
				Version:   "0.1.10",
				Artifacts: []UpdateArtifact{{OS: "linux", Arch: "amd64", URL: artifactURL, SHA256: sha256Hex(artifactBytes)}},
			})
			_ = jsonResponse(w, manifest)
		case "/bad-sha":
			manifest := signedManifest(t, privateKey, UpdateManifest{
				Version:   "0.1.10",
				Artifacts: []UpdateArtifact{{OS: "linux", Arch: "amd64", URL: artifactURL, SHA256: strings.Repeat("0", 64)}},
			})
			_ = jsonResponse(w, manifest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	artifactURL = server.URL + "/artifact"

	root := t.TempDir()
	exe := filepath.Join(root, "gpufleet-agent")
	if err := os.WriteFile(exe, []byte("old-agent-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	updater := AgentUpdater{CurrentVersion: "0.1.9", OS: "linux", Arch: "amd64", ExecutablePath: exe, StagingDir: root}
	if _, err := updater.ApplyPolicy(context.Background(), model.AgentUpdatePolicy{
		Enabled:        true,
		Mode:           "patch",
		DesiredVersion: "0.1.10",
		ManifestURL:    server.URL + "/bad-signature",
		PublicKey:      base64.StdEncoding.EncodeToString(otherPublicKey),
	}); err == nil {
		t.Fatal("expected bad signature to be rejected")
	}
	if _, err := updater.ApplyPolicy(context.Background(), model.AgentUpdatePolicy{
		Enabled:        true,
		Mode:           "patch",
		DesiredVersion: "0.1.10",
		ManifestURL:    server.URL + "/bad-sha",
		PublicKey:      base64.StdEncoding.EncodeToString(publicKey),
	}); err == nil {
		t.Fatal("expected bad sha256 to be rejected")
	}
	if got, err := os.ReadFile(exe); err != nil || string(got) != "old-agent-binary" {
		t.Fatalf("expected executable to remain unchanged, got %q err=%v", string(got), err)
	}
}

func signedManifest(t *testing.T, privateKey ed25519.PrivateKey, manifest UpdateManifest) UpdateManifest {
	t.Helper()
	payload, err := manifest.SigningBytes()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload))
	return manifest
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func jsonResponse(w http.ResponseWriter, value any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(value)
}
