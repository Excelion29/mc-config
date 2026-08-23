// Package security implementa los puertos Hasher y TokenGenerator de app.
package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Bcrypt implementa app.Hasher.
type Bcrypt struct {
	cost int
}

func NewBcrypt() *Bcrypt { return &Bcrypt{cost: bcrypt.DefaultCost} }

func (b *Bcrypt) Hash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(h), nil
}

func (b *Bcrypt) Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// RandomToken implementa app.TokenGenerator con 256 bits de entropia.
type RandomToken struct{}

func NewTokenGenerator() *RandomToken { return &RandomToken{} }

func (RandomToken) New() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("leyendo entropia: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
