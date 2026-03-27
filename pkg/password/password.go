// Package password provides bcrypt-based password hashing and verification.
// Copied from github.com/argoproj/argo-cd/v3/util/password to remove the dependency.
package password

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword creates a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("blank passwords are not allowed")
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// VerifyPassword checks whether a plaintext password matches a bcrypt hash.
// Returns (valid, stale) where stale is always false (single hasher).
func VerifyPassword(password, hashedPassword string) (bool, bool) {
	if password == "" {
		return false, false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil, false
}
