package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pbkdf2Iterations = 64_000
	saltLen          = 16
)

// HashPassword generates a random salt and hashes the password using PBKDF2-SHA256.
func HashPassword(password string) (saltHex, hashHex string, err error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}
	sum := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, 32, sha256.New)
	return hex.EncodeToString(salt), hex.EncodeToString(sum), nil
}

// CheckPassword compares the candidate password with the salt and hash in constant time.
func CheckPassword(password, saltHex, hashHex string) bool {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(hashHex)
	if err != nil || len(want) == 0 {
		return false
	}
	got := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, len(want), sha256.New)
	return subtle.ConstantTimeCompare(got, want) == 1
}
