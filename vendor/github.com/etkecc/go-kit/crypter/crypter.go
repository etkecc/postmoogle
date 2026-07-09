// Package crypter provides AES-GCM authenticated encryption and decryption for string values.
//
// # Format
//
// Every encrypted value is stored as a tagged string:
//
//	ENCv1[<base64url-raw(nonce || ciphertext || tag)>]
//
// Where:
//   - "ENCv1[" is the literal start tag (StartTag) identifying the format version.
//   - The payload is standard base64url without padding (RFC 4648 §5).
//   - The payload decodes to: 12-byte random nonce, followed by the AES-GCM ciphertext, followed by the 16-byte GCM authentication tag.
//   - "]" is the literal end tag (EndTag).
//
// # Key requirements
//
// The secret passed to New must be exactly 16, 24, or 32 bytes of raw key material
// (corresponding to AES-128, AES-192, or AES-256). It is used directly as the AES key
// with no stretching. Use a cryptographically random key (e.g. from pwgen -s or crypto/rand),
// not a human-memorable passphrase.
//
// # Mixed-plaintext configs
//
// Both Encrypt and Decrypt are safe to call on values that may or may not already be
// encrypted. Encrypt is idempotent (tagged values pass through unchanged); Decrypt
// returns untagged values as-is. This makes the package suitable as a transparent
// encrypt/decrypt layer over config maps that mix plaintext and encrypted fields.
//
// # Thread safety
//
// A single Crypter may be used concurrently from multiple goroutines without external
// synchronization. The internal nonce pool is goroutine-safe.
//
// # Usage
//
// Initialize once with a 16/24/32-byte key, then reuse the Crypter for all operations:
//
//	c, err := crypter.New(os.Getenv("SECRET_KEY")) // e.g. SECRET_KEY=$(pwgen -s 32 1)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Encrypt a plaintext value:
//
//	enc, err := c.Encrypt("my-database-password")
//	// enc == "ENCv1[<base64url>]"
//
// Decrypt it back:
//
//	plain, err := c.Decrypt(enc)
//	// plain == "my-database-password"
//
// Decrypt is safe to call on values that may or may not be encrypted — plaintext
// passes through unchanged, which makes it suitable as a transparent layer over
// config maps:
//
//	// both calls succeed; second returns "already-plain" as-is
//	c.Decrypt("ENCv1[...]")   // → decrypted plaintext, nil
//	c.Decrypt("already-plain") // → "already-plain", nil
//
// Check whether a value is encrypted without performing any cryptographic work:
//
//	if c.IsEncrypted(value) {
//	    // value came from an encrypted config field
//	}
package crypter

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	// StartTag is the prefix that identifies a string as encrypted by this package.
	// It encodes the format version ("v1") so future format changes can use a different prefix.
	// Any string passed to Encrypt that already begins with StartTag is assumed to be
	// encrypted and returned unchanged — callers must ensure plaintext values never start
	// with this prefix.
	//
	// Full format: ENCv1[<base64url-raw(nonce||ciphertext||tag)>]
	StartTag = "ENCv1["

	// EndTag is the suffix that closes the encrypted value.
	// It is a single "]" character, which is unambiguous within the base64url alphabet
	// (base64url uses A-Za-z0-9_-, none of which are "]").
	// Note: when embedding encrypted values in YAML or JSON, quote the value to prevent
	// parsers from interpreting the surrounding brackets as a flow sequence or array.
	EndTag = "]"
)

var (
	// ErrInvalidCipherText is returned when the ciphertext is structurally malformed:
	// missing the end tag, the decoded payload is shorter than a GCM nonce, or the
	// ciphertext was not produced by this package (wrong format version, truncated, etc.).
	// Authentication failures from a wrong key are reported as ErrOpen, not this error.
	ErrInvalidCipherText = errors.New("crypter: invalid ciphertext")

	// ErrInvalidKeyLength is returned by New when the provided secret is not 16, 24, or 32 bytes.
	// AES requires one of these exact key lengths (AES-128, AES-192, AES-256 respectively).
	// The length is measured in bytes, not Unicode code points — a 32-rune string with
	// multi-byte characters may be more than 32 bytes and will be rejected.
	ErrInvalidKeyLength = errors.New("crypter: invalid key length")

	// ErrEmptyPayload is returned when the tagged string contains no base64 payload,
	// i.e. the input is exactly "ENCv1[]". This is distinct from ErrInvalidCipherText
	// to make the failure mode explicit when debugging configuration values.
	ErrEmptyPayload = errors.New("crypter: empty payload")

	// ErrNewCipher is returned (wrapping the underlying error) when aes.NewCipher fails.
	// In practice this only happens if the key length is invalid, which New already
	// validates — so this error should never be seen in normal usage.
	ErrNewCipher = errors.New("crypter: aes.NewCipher failed")

	// ErrNewGCM is returned (wrapping the underlying error) when cipher.NewGCM fails.
	// This should never occur in practice; it is included for completeness.
	ErrNewGCM = errors.New("crypter: cipher.NewGCM failed")

	// ErrReadNonce is returned when reading random bytes from crypto/rand fails during
	// encryption. This is an OS-level error (e.g. /dev/urandom exhausted or unavailable)
	// and is unrecoverable. The encrypted value is not produced.
	ErrReadNonce = errors.New("crypter: read nonce failed")

	// ErrBase64Decode is returned when the payload between the tags is not valid base64url.
	// This means the ciphertext string was corrupted or hand-edited after encryption.
	ErrBase64Decode = errors.New("crypter: base64 decode failed")

	// ErrOpen is returned (wrapping the underlying cipher error) when AES-GCM
	// authentication or decryption fails. This happens when:
	//   - the ciphertext was encrypted with a different key,
	//   - the ciphertext or its authentication tag was tampered with, or
	//   - the nonce was corrupted.
	// The underlying error is wrapped so errors.Is(err, ErrOpen) works, and the
	// original cipher message is preserved for debugging via errors.Unwrap.
	ErrOpen = errors.New("crypter: aead open failed")
)

// startLen is the byte length of StartTag, cached to avoid repeated len() calls
// in the hot path of IsEncrypted.
const startLen = len(StartTag)

// Crypter encrypts and decrypts string values using AES-GCM authenticated encryption.
//
// A Crypter is initialized once with a fixed AES key and then reused for any number of
// Encrypt/Decrypt calls. The internal cipher and nonce pool are immutable after construction,
// so concurrent use is safe without external locking.
//
// The zero value is not usable; always construct with New.
type Crypter struct {
	// aead is the AES-GCM AEAD instance initialized from the caller's key.
	// It is safe for concurrent use; Go's cipher.AEAD implementations do not mutate
	// internal state during Seal/Open.
	aead cipher.AEAD

	// nonceSize is aead.NonceSize(), cached here to avoid repeated interface calls
	// in the hot path. For standard AES-GCM this is always 12 (96 bits).
	nonceSize int

	// noncePool is a pool of *[]byte nonce buffers, one per concurrent Encrypt call.
	// Using a pool avoids allocating a fresh 12-byte slice on every encryption.
	// The pool stores *[]byte (pointer) rather than []byte (slice header) so that
	// pool.Put never heap-allocates the interface box: a 24-byte slice header does not
	// fit in the pointer word of an interface value, but a pointer does.
	noncePool sync.Pool
}

// New initializes a Crypter with the provided AES key.
//
// The secret must be exactly 16, 24, or 32 bytes, selecting AES-128, AES-192, or AES-256
// respectively. The bytes are used directly as the AES key — no hashing or stretching is
// applied. Provide a cryptographically random key (e.g. from pwgen -s 32 or crypto/rand),
// not a human-memorable passphrase.
//
// Returns ErrInvalidKeyLength if the secret is not one of the three valid lengths.
// Other errors (ErrNewCipher, ErrNewGCM) indicate internal cipher initialization failure
// and should not occur with a valid key.
func New(secret string) (*Crypter, error) {
	key := []byte(secret)
	switch len(key) {
	case 16, 24, 32:
	default:
		return nil, ErrInvalidKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNewCipher, err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNewGCM, err)
	}

	nonceSize := aead.NonceSize()
	return &Crypter{
		aead:      aead,
		nonceSize: nonceSize,
		// Store *[]byte (pointer-sized) so pool.Put never heap-allocates the interface box.
		// A []byte header is 24 bytes and does not fit in the interface data word; a pointer does.
		noncePool: sync.Pool{New: func() any { b := make([]byte, nonceSize); return &b }},
	}, nil
}

// IsEncrypted reports whether s looks like a value encrypted by this package.
//
// It is a fast heuristic: it checks only that s begins with StartTag and has at least
// one byte after it. No base64 decoding or cryptographic verification is performed.
// A false positive (a plaintext value that happens to start with "ENCv1[") will cause
// Encrypt to skip encryption and Decrypt to attempt — and fail — decryption.
//
// Contract: plaintext values must never begin with StartTag. This is the caller's
// responsibility; the package does not enforce it.
func (c *Crypter) IsEncrypted(s string) bool {
	return len(s) > startLen && s[:startLen] == StartTag
}

// StartTag returns the StartTag constant: the prefix that marks a value as
// encrypted by this package.
//
// It lets callers obtain the tag through an interface method instead of importing
// the constant. The crypter/yaml module relies on this: it defines its own
// Crypter interface and imports nothing from this package, so *Crypter exposes the
// tag as a method to satisfy that interface and drive its decrypt fast-path.
func (c *Crypter) StartTag() string {
	return StartTag
}

// Encrypt encrypts data using AES-GCM and returns the result wrapped in ENCv1[...].
//
// If data is already tagged (IsEncrypted returns true), it is returned unchanged —
// Encrypt is idempotent. Otherwise a fresh 12-byte random nonce is drawn from
// crypto/rand, the plaintext is sealed with AES-GCM, and the result is encoded as:
//
//	ENCv1[<base64url-raw(nonce || ciphertext || GCM-tag)>]
//
// The nonce is unique per call, so encrypting the same plaintext twice produces
// different ciphertext each time.
//
// Warning: if data begins with StartTag but is not a valid encrypted value (a violation
// of the caller contract documented on IsEncrypted), Encrypt returns it unchanged without
// error. The caller would silently store plaintext while believing it to be encrypted.
// Always ensure plaintext values cannot begin with StartTag.
//
// Returns ErrReadNonce if crypto/rand is unavailable (OS-level failure).
func (c *Crypter) Encrypt(data string) (string, error) {
	if c.IsEncrypted(data) {
		return data, nil
	}

	noncep, ok := c.noncePool.Get().(*[]byte)
	if !ok {
		// should never happen; pool.New always returns *[]byte
		b := make([]byte, c.nonceSize)
		noncep = &b
	}
	if _, err := io.ReadFull(rand.Reader, *noncep); err != nil {
		c.noncePool.Put(noncep)
		return "", fmt.Errorf("%w: %w", ErrReadNonce, err)
	}
	// Copy random bytes into an owned slice, then return the pool buffer immediately.
	// Seal and all code below only touch this owned slice — the pool buffer is
	// unreachable from this point, eliminating any concern about pool lifetime or
	// concurrent reuse of its backing array.
	nonce := make([]byte, c.nonceSize)
	copy(nonce, *noncep)
	c.noncePool.Put(noncep)

	// Pre-allocate raw with nonce prefix, then let Seal append ciphertext+tag in place.
	// dst (raw) and nonce are distinct allocations — no aliasing.
	raw := make([]byte, 0, c.nonceSize+len(data)+c.aead.Overhead())
	raw = append(raw, nonce...)
	raw = c.aead.Seal(raw, nonce, []byte(data), nil)

	encodedLen := base64.RawURLEncoding.EncodedLen(len(raw))
	buf := make([]byte, startLen, startLen+encodedLen+len(EndTag))
	copy(buf, StartTag)
	buf = base64.RawURLEncoding.AppendEncode(buf, raw)
	buf = append(buf, EndTag...)
	return string(buf), nil
}

// Decrypt decrypts a tagged value and returns the original plaintext.
//
// If data is not tagged (IsEncrypted returns false), it is returned unchanged without
// error — this is intentional for configs that mix plaintext and encrypted values.
// If you need to enforce that a value is always encrypted, check IsEncrypted before calling.
//
// For tagged values, the base64 payload is decoded, the 12-byte nonce is extracted from
// the front, and AES-GCM Open is called to authenticate and decrypt the ciphertext.
//
// Error cases for tagged input:
//   - ErrInvalidCipherText: missing end tag, decoded payload shorter than a GCM nonce,
//     or payload too short to contain the GCM authentication tag.
//   - ErrEmptyPayload: the tag is present but contains no base64 content ("ENCv1[]").
//   - ErrBase64Decode: the payload is not valid base64url.
//   - ErrOpen: AES-GCM authentication failed — wrong key, corrupted ciphertext, or tampered tag.
func (c *Crypter) Decrypt(data string) (string, error) {
	if !c.IsEncrypted(data) {
		return data, nil
	}

	// Validate end tag and extract the base64url payload.
	if data[len(data)-1] != EndTag[0] {
		return "", ErrInvalidCipherText
	}
	payload := data[startLen : len(data)-1] // slice only, no alloc
	if payload == "" {
		return "", ErrEmptyPayload
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrBase64Decode, err)
	}

	if len(raw) < c.nonceSize {
		return "", ErrInvalidCipherText
	}

	nonce, ct := raw[:c.nonceSize], raw[c.nonceSize:]
	if len(ct) < c.aead.Overhead() {
		// Payload has a valid nonce but no room for the GCM authentication tag.
		return "", ErrInvalidCipherText
	}
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		// Map all auth/decrypt failures to a stable sentinel, but keep original for debugging via wrapping.
		return "", fmt.Errorf("%w: %w", ErrOpen, err)
	}
	return string(pt), nil
}
