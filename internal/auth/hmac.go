package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	HeaderDeviceID  = "X-GF-Device-Id"
	HeaderTimestamp = "X-GF-Timestamp"
	HeaderNonce     = "X-GF-Nonce"
	HeaderSignature = "X-GF-Signature"
)

func Sign(method, path string, body []byte, deviceID, secret string, at time.Time, nonce string) string {
	sum := sha256.Sum256(body)
	signingString := strings.Join([]string{
		strings.ToUpper(method),
		path,
		deviceID,
		at.UTC().Format(time.RFC3339),
		nonce,
		hex.EncodeToString(sum[:]),
	}, "\n")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func Verify(method, path string, body []byte, deviceID, timestamp, nonce, signature, secret string, now time.Time, maxSkew time.Duration) error {
	if deviceID == "" {
		return errors.New("missing device id")
	}
	if timestamp == "" || nonce == "" || signature == "" {
		return errors.New("missing authentication headers")
	}
	at, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if at.Before(now.Add(-maxSkew)) || at.After(now.Add(maxSkew)) {
		return errors.New("timestamp outside allowed window")
	}

	expected := Sign(method, path, body, deviceID, secret, at, nonce)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return errors.New("signature mismatch")
	}
	return nil
}

func AttachSignedHeaders(req *http.Request, body []byte, deviceID, secret string, at time.Time) error {
	nonce, err := RandomToken(18)
	if err != nil {
		return err
	}
	req.Header.Set(HeaderDeviceID, deviceID)
	req.Header.Set(HeaderTimestamp, at.UTC().Format(time.RFC3339))
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, Sign(req.Method, req.URL.EscapedPath(), body, deviceID, secret, at, nonce))
	return nil
}

func RandomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
