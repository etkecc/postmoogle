package zlogsentry

import (
	"errors"
)

var ErrFlushTimeout = errors.New("zlogsentry flush timeout")
