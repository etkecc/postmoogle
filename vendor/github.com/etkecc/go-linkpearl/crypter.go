package linkpearl

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"

	"github.com/etkecc/go-kit/crypter"
)

// Crypter encrypts account data with a secret key, apart from Matrix' standard
// encryption. It writes go-kit's ENCv1[...] and reads that plus the old StdBase64 format.
type Crypter struct {
	gokit     *crypter.Crypter // writes/reads ENCv1[...]
	legacy    cipher.AEAD      // old StdBase64 AES-GCM, decrypt-only
	nonceSize int
}

// ErrInvalidData is returned when the provided encrypted data (ciphertext) is invalid.
var ErrInvalidData = errors.New("invalid data")

// NewCrypter creates new Crypter. go-kit validates the 16/24/32-byte key first, so the
// legacy AEAD cannot fail on length.
func NewCrypter(secretkey string) (*Crypter, error) {
	gokit, err := crypter.New(secretkey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(secretkey))
	if err != nil {
		return nil, err
	}
	legacy, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Crypter{gokit: gokit, legacy: legacy, nonceSize: legacy.NonceSize()}, nil
}

// Decrypt data. Route via go-kit's own IsEncrypted, never a hand-rolled prefix check:
// a gate one byte looser waves "ENCv1[" through as fake plaintext with a straight face.
func (c *Crypter) Decrypt(data string) (string, error) {
	if c.gokit.IsEncrypted(data) {
		plain, err := c.gokit.Decrypt(data)
		if err != nil {
			return data, err // return input, not "": caller stores this even on error, so plaintext-era values survive (accountdata.go:116)
		}
		return plain, nil
	}

	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return data, err
	}
	if len(raw) < c.nonceSize {
		return data, ErrInvalidData
	}
	nonce, ciphertext := raw[:c.nonceSize], raw[c.nonceSize:]
	plain, err := c.legacy.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return data, err
	}
	return string(plain), nil
}

// Encrypt data into ENCv1[...] format. Idempotent: an already-tagged value passes through.
func (c *Crypter) Encrypt(data string) (string, error) {
	enc, err := c.gokit.Encrypt(data)
	if err != nil {
		return data, err
	}
	return enc, nil
}
