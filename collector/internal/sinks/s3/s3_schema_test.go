package s3

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestWriteEventAppliesSchemaProjection(t *testing.T) {
	s := &sink{
		cfg: Config{
			Schema: map[string]string{
				"id":     "event_id",
				"status": "status_code",
			},
		},
		columns:   []string{"id", "status"},
		batchSize: 10,
	}

	err := s.WriteEvent(context.Background(), []byte(`{"event_id":"e1","status_code":200,"ignored":"x"}`), nil)
	if err != nil {
		t.Fatalf("write event failed: %v", err)
	}

	got := s.buf.String()
	if !strings.Contains(got, `"id":"e1"`) || !strings.Contains(got, `"status":200`) {
		t.Fatalf("projected payload missing fields: %s", got)
	}
	if strings.Contains(got, "ignored") {
		t.Fatalf("payload still contains ignored field: %s", got)
	}
}

func TestNewRejectsInvalidSchemaPath(t *testing.T) {
	_, err := New(Config{
		Client: &s3.Client{},
		Bucket: "bucket",
		Schema: map[string]string{
			"id": "event..id",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid schema path") {
		t.Fatalf("expected invalid schema path error, got: %v", err)
	}
}

func TestNewDefaultObjectKeyUsesArchiveConvention(t *testing.T) {
	gotSink, err := New(Config{
		Client: &s3.Client{},
		Bucket: "bucket",
		Prefix: "archive/raw",
	})
	if err != nil {
		t.Fatalf("new sink failed: %v", err)
	}
	s := gotSink.(*sink)
	t.Cleanup(func() { _ = s.Close(context.Background()) })

	key := s.cfg.ObjectKeyFn()
	re := regexp.MustCompile(`^archive/raw/dataset=events/date=\d{4}-\d{2}-\d{2}/hour=\d{2}/part-\d+\.ndjson$`)
	if !re.MatchString(key) {
		t.Fatalf("unexpected key format: %s", key)
	}
}
