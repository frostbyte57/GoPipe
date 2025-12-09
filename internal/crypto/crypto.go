package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/secretbox"
)

func DeriveKey(secret []byte, salt []byte, info string) []byte {
	hkdf := hkdf.New(sha256.New, secret, salt, []byte(info))
	key := make([]byte, 32)
	io.ReadFull(hkdf, key)
	return key
}

func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: %d", len(key))
	}

	var secretKey [32]byte
	copy(secretKey[:], key)

	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// seal appends to out, so we start with nonce to be friendly
	encrypted := secretbox.Seal(nonce[:], plaintext, &nonce, &secretKey)
	return encrypted, nil
}

func Decrypt(key []byte, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: %d", len(key))
	}
	if len(ciphertext) < 24 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	var secretKey [32]byte
	copy(secretKey[:], key)

	var nonce [24]byte
	copy(nonce[:], ciphertext[:24])

	decrypted, ok := secretbox.Open(nil, ciphertext[24:], &nonce, &secretKey)
	if !ok {
		return nil, fmt.Errorf("decryption failed")
	}
	return decrypted, nil
}

func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, b)
	return b, err
}
