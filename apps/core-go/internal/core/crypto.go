package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

func digestToken(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:])
}

// verifyArgon2ID verifies the PasswordHasher format emitted by argon2-cffi.
func verifyArgon2ID(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, iterations, parallelism uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		salt, err = base64.StdEncoding.DecodeString(parts[4])
		if err != nil {
			return false
		}
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		want, err = base64.StdEncoding.DecodeString(parts[5])
		if err != nil {
			return false
		}
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(want)))
	if len(got) != len(want) {
		return false
	}
	var mismatch byte
	for index := range got {
		mismatch |= got[index] ^ want[index]
	}
	return mismatch == 0
}

func hashArgon2ID(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	const memory = 65536
	const iterations = 3
	const parallelism = 4
	const keyLength = 32
	digest := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations, parallelism, encode(salt), encode(digest)), nil
}

func decodeSettingsKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("FLUCTLIGHT_SETTINGS_KEY must be base64-encoded 32 bytes")
	}
	return key, nil
}

func decryptSecret(key []byte, purpose string, nonce, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	var gcm cipher.AEAD
	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(purpose))
	if err != nil {
		return "", errors.New("settings secret authentication failed")
	}
	return string(plain), nil
}
