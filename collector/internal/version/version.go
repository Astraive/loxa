package version

import (
	"os"
	"strings"
	"sync"
)

const fallbackVersion = "0.3.2"

var (
	once    sync.Once
	version string
)

// CollectorVersion returns the collector version from loza.yaml,
// falling back to the hardcoded default if the file cannot be found or parsed.
func CollectorVersion() string {
	once.Do(func() {
		version = loadVersion()
	})
	return version
}

func loadVersion() string {
	candidates := []string{
		"loza.yaml",
		"../loza.yaml",
		"../../loza.yaml",
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
