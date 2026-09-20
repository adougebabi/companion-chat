package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
)

func DecodeSettingsKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, errors.New("FLUCTLIGHT_SETTINGS_KEY must be base64-encoded 32 bytes")
	}
	return key, nil
}

func DecryptSecret(key []byte, purpose string, nonce, ciphertext []byte) (string, error) {
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
