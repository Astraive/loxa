package storagepath

import (
	"testing"
	"time"
)

func TestStoragePathSanitizationDefaults(t *testing.T) {
	if got := normalizePrefix(" /archive\\raw/ "); got != "archive/raw/" {
		t.Fatalf("normalize prefix = %q", got)
	}
	if got := normalizePrefix("///"); got != "" {
		t.Fatalf("empty prefix = %q", got)
	}
	if got := sanitizeSegment(" "); got != "events" {
		t.Fatalf("empty segment = %q", got)
	}
	if got := sanitizeExt(" "); got != "ndjson" {
		t.Fatalf("empty extension = %q", got)
	}
	if got := sanitizeExt(".parquet\\"); got != "parquet" {
		t.Fatalf("sanitized extension = %q", got)
	}
	_ = NDJSONArchiveKey("", time.Unix(0, 0))
}
