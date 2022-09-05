package utils

import (
	"fmt"
	"strings"
)

// ErrInvalidArgs returned when a command's arguments are invalid
var ErrInvalidArgs = fmt.Errorf("invalid arguments")

// ParseSend parses "!pm send" command, returns to, subject, body, err
func ParseSend(commandSlice []string) (string, string, string, error) {
	if len(commandSlice) < 3 {
		return "", "", "", ErrInvalidArgs
	}

	message := strings.Join(commandSlice, " ")
	lines := strings.Split(message, "\n")
	commandSlice = strings.Split(lines[0], " ")
	to := commandSlice[1]
	subject := lines[1]
	body := strings.Join(lines[2:], "\n")

	return to, subject, body, nil
}
