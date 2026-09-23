package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// signatureHeader is the header river-events sets on every delivery.
const signatureHeader = "X-River-Signature"

// eventHeader carries the routing key ("<kind>.<type>") for cheap pre-filtering.
const eventHeader = "X-River-Event"

// sign computes the HMAC-SHA256 of body under secret, hex-encoded — the exact
// value river-events puts in X-River-Signature (without the "sha256=" prefix).
func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// validSignature reports whether the "sha256=<hex>" header matches the HMAC of
// body under secret, using a constant-time compare. A missing/misformatted
// header is invalid.
func validSignature(header, secret string, body []byte) bool {
	got, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	want := sign(secret, body)
	return hmac.Equal([]byte(got), []byte(want))
}
