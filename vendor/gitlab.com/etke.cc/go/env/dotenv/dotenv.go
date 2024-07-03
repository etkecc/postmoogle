package dotenv

import (
	"errors"
	"os"
)

// EnvFile is the default file to load
const EnvFile = ".env"

// Load loads the EnvFile and additional files
func Load(additionalFiles ...string) {
	files := []string{EnvFile}
	if len(additionalFiles) > 0 {
		files = append(files, additionalFiles...)
	}

	for _, file := range files {
		loadFile(file) //nolint:errcheck // ignore error
	}
}

func loadFile(file string) error {
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	contents, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	contentsMap := make(map[string]string)
	if err := parseBytes(contents, contentsMap); err != nil {
		return err
	}

	for key, value := range contentsMap {
		os.Setenv(key, value)
	}

	return nil
}
