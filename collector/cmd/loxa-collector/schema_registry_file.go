package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	collectorconfig "github.com/astraive/loxa-collector/internal/config"
)

func loadSchemaRegistryFile(fc *fileConfig) error {
	path := strings.TrimSpace(fc.Schema.RegistryFile)
	if path == "" {
		return nil
	}
	entries, err := loadPersistedSchemaRegistry(path, fc.Storage.EncryptionKey)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load schema registry file: %w", err)
	}
	fc.Schema.Registry = entries
	return nil
}

func saveSchemaRegistryFile(path string, entries []collectorconfig.SchemaRegistryEntry, encryptionKey string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if strings.TrimSpace(encryptionKey) != "" {
		data, err = encryptBlob(data, encryptionKey)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadPersistedSchemaRegistry(path, encryptionKey string) ([]collectorconfig.SchemaRegistryEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(encryptionKey) != "" {
		raw, err = decryptBlob(raw, encryptionKey)
		if err != nil {
			return nil, err
		}
	}
	var entries []collectorconfig.SchemaRegistryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
