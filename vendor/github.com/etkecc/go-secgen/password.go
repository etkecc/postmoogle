package secgen

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"strings"
)

const charset = "abcdedfghijklmnopqrstABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // a-z A-Z 0-9
var charsetlen = big.NewInt(57)

// Password generates secure password
func Password(length int) string {
	var password strings.Builder

	for i := 0; i < length; i++ {
		index, _ := rand.Int(rand.Reader, charsetlen)
		password.WriteByte(charset[index.Int64()])
	}

	return password.String()
}

// Base64Bytes generates secure bytes with the given length and returns it as a base64 string
func Base64Bytes(length int) string {
	randomBytes := make([]byte, length)
	rand.Read(randomBytes) //nolint:errcheck // nothing could be done anyway
	return base64.StdEncoding.EncodeToString(randomBytes)
}
