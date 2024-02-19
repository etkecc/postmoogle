# zerolog-sentry
[![Build Status](https://github.com/archdx/zerolog-sentry/workflows/test/badge.svg)](https://github.com/archdx/zerolog-sentry/actions)
[![codecov](https://codecov.io/gh/archdx/zerolog-sentry/branch/master/graph/badge.svg)](https://codecov.io/gh/archdx/zerolog-sentry)

### Example
```go
import (
	"errors"
	stdlog "log"
	"os"

	"github.com/archdx/zerolog-sentry"
	"github.com/rs/zerolog"
)

func main() {
	w, err := zlogsentry.New("http://e35657dcf4fb4d7c98a1c0b8a9125088@localhost:9000/2", zlogsentry.WithEnvironment("dev"), zlogsentry.WithRelease("1.0.0"))
	if err != nil {
		stdlog.Fatal(err)
	}

	defer w.Close()

	multi := zerolog.MultiLevelWriter(os.Stdout, w)
	logger := zerolog.New(multi).With().Timestamp().Logger()

	logger.Error().Err(errors.New("dial timeout")).Msg("test message")
}

```
