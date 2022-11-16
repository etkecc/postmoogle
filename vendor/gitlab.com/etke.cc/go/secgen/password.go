package secgen

import (
	"crypto/rand"
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
