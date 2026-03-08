package contextx

import (
	"os"
)

// ReadFileLimited reads up to maxBytes from the specified file path.
func ReadFileLimited(path string, maxBytes int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	return string(data), nil
}

