package projection

import "testing"

func TestExtractPathSupportsArrayIndices(t *testing.T) {
	doc, err := DecodeObject([]byte(`{"checkpoints":[{"name":"start"},{"name":"end"}]}`))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	got, ok := ExtractPath(doc, "checkpoints[1].name")
	if !ok || got != "end" {
		t.Fatalf("expected checkpoints[1].name=end, got %v (%v)", got, ok)
	}
}

func TestValidatePathRejectsMalformedPaths(t *testing.T) {
	cases := []string{
		"",
		"user..id",
		"user.",
		"user[abc]",
		"user[]",
		"user]",
	}
	for _, tc := range cases {
		if err := ValidatePath(tc); err == nil {
			t.Fatalf("expected invalid path error for %q", tc)
		}
	}
}

func TestValidatePathAcceptsMixedSegments(t *testing.T) {
	if err := ValidatePath("events[0].attrs.user.id"); err != nil {
		t.Fatalf("expected valid path, got: %v", err)
	}
}
