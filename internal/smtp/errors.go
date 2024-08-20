package smtp

import (
	"errors"
	"strings"

	"github.com/emersion/go-smtp"
)

const (
	// NoUserCode SMTP code
	NoUserCode = 550
	// BannedCode SMTP code
	BannedCode = 554
	// GreylistCode SMTP code
	GreylistCode = 451
	// RBLCode SMTP code
	RBLCode = 450
)

var (
	// NoUserEnhancedCode enhanced SMTP code
	NoUserEnhancedCode = smtp.EnhancedCode{5, 5, 0}
	// BannedEnhancedCode enhanced SMTP code
	BannedEnhancedCode = smtp.EnhancedCode{5, 5, 4}
	// GreylistEnhancedCode is GraylistCode in enhanced code notation
	GreylistEnhancedCode = smtp.EnhancedCode{4, 5, 1}
	// RBLEnhancedCode is RBLCode in enhanced code notation
	RBLEnhancedCode = smtp.EnhancedCode{4, 5, 0}
	// ErrBanned returned to banned hosts
	ErrBanned = &smtp.SMTPError{
		Code:         BannedCode,
		EnhancedCode: BannedEnhancedCode,
		Message:      "please, don't bother me anymore, kupo.",
	}
	// ErrNoUser returned when no such mailbox found
	ErrNoUser = &smtp.SMTPError{
		Code:         NoUserCode,
		EnhancedCode: NoUserEnhancedCode,
		Message:      "no such user here, kupo.",
	}
	// ErrGreylisted returned when the host is graylisted
	ErrGreylisted = &smtp.SMTPError{
		Code:         GreylistCode,
		EnhancedCode: GreylistEnhancedCode,
		Message:      "You have been greylisted, try again a bit later.",
	}
	// ErrRBL returned when the host is blacklisted
	ErrRBL = &smtp.SMTPError{
		Code:         RBLCode,
		EnhancedCode: RBLEnhancedCode,
		Message:      "You are blacklisted, kupo.",
	}
	// ErrInvalidEmail for invalid emails :)
	ErrInvalidEmail = errors.New("please, provide valid email address")
)

// extendErrRBL extends the RBL error with reasons returned by the DNSBLs
func extendErrRBL(reasons []string) *smtp.SMTPError {
	return &smtp.SMTPError{
		Code:         RBLCode,
		EnhancedCode: RBLEnhancedCode,
		Message:      "You are blacklisted, kupo. Details: " + strings.Join(reasons, "; "),
	}
}
