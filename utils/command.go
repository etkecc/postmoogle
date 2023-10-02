package utils

import (
	"fmt"
	"strings"
)

// MinSendCommandParts is minimal count of space-separated parts for !pm send command
const MinSendCommandParts = 3

// ErrInvalidArgs returned when a command's arguments are invalid
var ErrInvalidArgs = fmt.Errorf("invalid arguments")

// ParseSend parses "!pm send" command, returns to, subject, body, err
func ParseSend(commandSlice []string) (to, subject, body string, err error) {
	message := strings.Join(commandSlice, " ")
	lines := strings.Split(message, "\n")
	if len(lines) < MinSendCommandParts {
		return "", "", "", ErrInvalidArgs
	}

	commandSlice = strings.Split(lines[0], " ")
	to = commandSlice[1]
	subject = lines[1]
	body = strings.Join(lines[2:], "\n")

	return to, subject, body, nil
}
