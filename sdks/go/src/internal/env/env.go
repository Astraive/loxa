package env

import (
	"os"
	"strconv"
)

// GetString returns the value of the named environment variable,
// or def if the variable is unset or empty.
func GetString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// GetBool returns the boolean value of the named env var, or def if unset.
func GetBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// GetInt returns the integer value of the named env var, or def if unset.
func GetInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}
