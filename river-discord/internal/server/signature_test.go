package server

import "testing"

func TestValidSignature(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"kind":"media.ready","type":"movie","media_id":"m1"}`)
	good := "sha256=" + sign(secret, body)

	tests := []struct {
		name   string
		header string
		secret string
		body   []byte
		want   bool
	}{
		{"valid", good, secret, body, true},
		{"wrong secret", good, "other", body, false},
		{"tampered body", good, secret, append(body, '!'), false},
		{"missing prefix", sign(secret, body), secret, body, false},
		{"empty header", "", secret, body, false},
		{"garbage", "sha256=zzzz", secret, body, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validSignature(tt.header, tt.secret, tt.body); got != tt.want {
				t.Fatalf("validSignature = %v, want %v", got, tt.want)
			}
		})
	}
}
