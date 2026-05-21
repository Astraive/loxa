package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// NDJSONArchiveKey returns a partitioned object-storage key for NDJSON archives.
func NDJSONArchiveKey(prefix string, ts time.Time) string {
	return archiveObjectKey(prefix, "events", "ndjson", ts)
}

// ParquetArchiveKey returns a partitioned object-storage key for Parquet archives.
func ParquetArchiveKey(prefix, table string, ts time.Time) string {
	return archiveObjectKey(prefix, sanitizeSegment(table), "parquet", ts)
}

// LocalParquetExportPath returns a partitioned filesystem path for Parquet exports.
func LocalParquetExportPath(baseDir, table string, ts time.Time) string {
	utc := ts.UTC()
	return filepath.Join(
		baseDir,
		"table="+sanitizeSegment(table),
		"date="+utc.Format("2006-01-02"),
		"hour="+utc.Format("15"),
		fmt.Sprintf("part-%d.parquet", utc.UnixNano()),
	)
}

func archiveObjectKey(prefix, dataset, ext string, ts time.Time) string {
	utc := ts.UTC()
	return fmt.Sprintf(
		"%sdataset=%s/date=%s/hour=%s/part-%d.%s",
		normalizePrefix(prefix),
		sanitizeSegment(dataset),
		utc.Format("2006-01-02"),
		utc.Format("15"),
		utc.UnixNano(),
		sanitizeExt(ext),
	)
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.ReplaceAll(prefix, "\\", "/")
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return ""
	}
	return prefix + "/"
}

func sanitizeSegment(seg string) string {
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return "events"
	}
	seg = strings.ReplaceAll(seg, "\\", "_")
	seg = strings.ReplaceAll(seg, "/", "_")
	seg = strings.ReplaceAll(seg, ".", "_")
	return seg
}

func sanitizeExt(ext string) string {
	ext = strings.TrimSpace(ext)
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		return "ndjson"
	}
	ext = strings.ReplaceAll(ext, "\\", "")
	ext = strings.ReplaceAll(ext, "/", "")
	return ext
}
