package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const MinSecretLength = 32

type Payload struct {
	RoomID   string `json:"room_id"`
	Number   int64  `json:"number"`
	IssuedAt int64  `json:"issued_at"`
}

// sentinel errors
var (
	ErrMalformed    = errors.New("malformed")
	ErrBadSignature = errors.New("bad signature")
)

type Signer struct {
	secret []byte
}

func NewSigner(secret string) (*Signer, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("secret must be atleast %d characters long", MinSecretLength)
	}

	return &Signer{secret: []byte(secret)}, nil
}

func (s *Signer) Sign(p Payload) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(data)

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payloadPart))
	sig := mac.Sum(nil)

	sigPart := base64.RawURLEncoding.EncodeToString(sig)
	return payloadPart + "." + sigPart, nil
}

func (s *Signer) Verify(token string) (Payload, error) {
	payloadPart, sigPart, found := strings.Cut(token, ".")
	if !found || payloadPart == "" || sigPart == "" || strings.Contains(sigPart, ".") {
		return Payload{}, ErrMalformed
	}

	gotSig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return Payload{}, ErrMalformed
	}

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payloadPart))
	wantSig := mac.Sum(nil)
	if !hmac.Equal(gotSig, wantSig) {
		return Payload{}, ErrBadSignature
	}

	data, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return Payload{}, ErrMalformed
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, ErrMalformed
	}
	return p, nil
}
