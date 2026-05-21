package storagepath

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNDJSONArchiveKey(t *testing.T) {
	ts := time.Date(2026, time.March, 9, 14, 5, 6, 123456789, time.UTC)
	got := NDJSONArchiveKey("archive/raw", ts)
	want := "archive/raw/dataset=events/date=2026-03-09/hour=14/part-1773065106123456789.ndjson"
	if got != want {
		t.Fatalf("unexpected key:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestParquetArchiveKeySanitizesTable(t *testing.T) {
	ts := time.Date(2026, time.March, 9, 14, 5, 6, 123456789, time.UTC)
	got := ParquetArchiveKey("", `analytics\wide.events`, ts)
	want := "dataset=analytics_wide_events/date=2026-03-09/hour=14/part-1773065106123456789.parquet"
	if got != want {
		t.Fatalf("unexpected key:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestLocalParquetExportPath(t *testing.T) {
	ts := time.Date(2026, time.March, 9, 14, 5, 6, 123456789, time.UTC)
	got := filepath.ToSlash(LocalParquetExportPath("exports", "analytics.events", ts))
	want := "exports/table=analytics_events/date=2026-03-09/hour=14/part-1773065106123456789.parquet"
	if got != want {
		t.Fatalf("unexpected path:\nwant: %s\ngot:  %s", want, got)
	}
}
