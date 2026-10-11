package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestSigner(t *testing.T, secret string) *Signer {
	t.Helper()
	s, err := NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func TestNewSignerShortSecret(t *testing.T) {
	if _, err := NewSigner("too-short"); err == nil {
		t.Fatal("expected error for short secret, got nil")
	}
	if _, err := NewSigner(testSecret); err != nil {
		t.Fatalf("32-byte secret rejected: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	s := newTestSigner(t, testSecret)
	in := Payload{RoomID: "demo", Number: 7, IssuedAt: 1760000000}

	tok, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	out, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if out != in {
		t.Fatalf("got %+v, want %+v", out, in)
	}
}

func TestSignIsDeterministic(t *testing.T) {
	s := newTestSigner(t, testSecret)
	p := Payload{RoomID: "demo", Number: 7, IssuedAt: 1760000000}
	a, _ := s.Sign(p)
	b, _ := s.Sign(p)
	if a != b {
		t.Fatal("same payload and secret produced different tokens")
	}
}

func TestVerifyTamperedPayload(t *testing.T) {
	s := newTestSigner(t, testSecret)
	tok7, _ := s.Sign(Payload{RoomID: "demo", Number: 7, IssuedAt: 1})
	tok1, _ := s.Sign(Payload{RoomID: "demo", Number: 1, IssuedAt: 1})

	// payload of number 1 with the signature of number 7
	payload1, _, _ := strings.Cut(tok1, ".")
	_, sig7, _ := strings.Cut(tok7, ".")

	_, err := s.Verify(payload1 + "." + sig7)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestVerifyTamperedSignature(t *testing.T) {
	s := newTestSigner(t, testSecret)
	tok, _ := s.Sign(Payload{RoomID: "demo", Number: 7, IssuedAt: 1})
	payload, _, _ := strings.Cut(tok, ".")

	fakeSig := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	_, err := s.Verify(payload + "." + fakeSig)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	a := newTestSigner(t, testSecret)
	b := newTestSigner(t, "ffffffffffffffffffffffffffffffff")

	tok, _ := a.Sign(Payload{RoomID: "demo", Number: 7, IssuedAt: 1})
	_, err := b.Verify(tok)
	if !errors.Is(err, ErrBadSignature) {
		t.Fatalf("got %v, want ErrBadSignature", err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	s := newTestSigner(t, testSecret)
	cases := map[string]string{
		"empty":           "",
		"no dot":          "abc",
		"three parts":     "a.b.c",
		"empty payload":   ".abc",
		"empty signature": "abc.",
		"bad base64 sig":  "abc.!!!",
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := s.Verify(tok)
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("got %v, want ErrMalformed", err)
			}
		})
	}
}

// A correctly signed payload that isn't valid JSON must be ErrMalformed,
// not a panic or a bad signature.
func TestVerifySignedButNotJSON(t *testing.T) {
	s := newTestSigner(t, testSecret)

	payloadPart := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payloadPart))
	sigPart := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	_, err := s.Verify(payloadPart + "." + sigPart)
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("got %v, want ErrMalformed", err)
	}
}
