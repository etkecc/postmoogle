package argon2pw

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordType = "argon2id"
	saltLen      = 32
	argon2KeyLen = 32
	argon2Time   = 1
	argon2Memory = 64 * 1024
)

var argon2Threads = uint8(runtime.NumCPU())

// GenerateSaltedHash takes a plaintext password and generates an argon2 hash
func GenerateSaltedHash(password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("Password length cannot be 0")
	}
	salt, _ := generateSalt(saltLen)
	unencodedPassword := argon2.IDKey([]byte(password), []byte(salt), argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	encodedPassword := base64.StdEncoding.EncodeToString(unencodedPassword)
	hash := fmt.Sprintf("%s$%d$%d$%d$%d$%s$%s",
		passwordType, argon2Time, argon2Memory, argon2Threads, argon2KeyLen, salt, encodedPassword)

	return hash, nil
}

// CompareHashWithPassword compares an argon2 hash against plaintext password
func CompareHashWithPassword(hash, password string) (bool, error) {
	if len(hash) == 0 || len(password) == 0 {
		return false, errors.New("Arguments cannot be zero length")
	}
	hashParts := strings.Split(hash, "$")
	if len(hashParts) != 7 {
		return false, errors.New("Invalid Password Hash")
	}

	passwordType := hashParts[0]
	time, _ := strconv.Atoi((hashParts[1]))
	memory, _ := strconv.Atoi(hashParts[2])
	threads, _ := strconv.Atoi(hashParts[3])
	keyLen, _ := strconv.Atoi(hashParts[4])
	salt := []byte(hashParts[5])
	key, _ := base64.StdEncoding.DecodeString(hashParts[6])

	var calculatedKey []byte
	switch passwordType {
	case "argon2id":
		calculatedKey = argon2.IDKey([]byte(password), salt, uint32(time), uint32(memory), uint8(threads), uint32(keyLen))
	case "argon2i", "argon2":
		calculatedKey = argon2.Key([]byte(password), salt, uint32(time), uint32(memory), uint8(threads), uint32(keyLen))
	default:
		return false, errors.New("Invalid Password Hash")
	}

	if subtle.ConstantTimeCompare(key, calculatedKey) != 1 {
		return false, errors.New("Password did not match")
	}
	return true, nil
}

func generateSalt(len int) (string, error) {
	unencodedSalt := make([]byte, len)
	if _, err := rand.Read(unencodedSalt); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(unencodedSalt), nil
}
