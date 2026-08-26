package core

import (
	"os"
	"strings"
	"sync"
)

const fallbackVersion = "0.3.4"

var (
	sdkVersionOnce sync.Once
	sdkVersionVal  string
)

// SDKVersion returns the SDK version from loza-go.yaml, falling back to
// the hardcoded default if the file cannot be found or parsed.
func SDKVersion() string {
	sdkVersionOnce.Do(func() {
		sdkVersionVal = loadVersion()
	})
	return sdkVersionVal
}

func loadVersion() string {
	candidates := []string{
		"loza-go.yaml",
		"../loza-go.yaml",
		"../../loza-go.yaml",
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "version:") {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
				value = strings.Trim(value, "\"'")
				if value != "" {
					return value
				}
			}
		}
	}

	return fallbackVersion
}
