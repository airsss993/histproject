package submissions

import (
	"strings"
	"testing"
)

func TestSubmissionRateKeyContainsOnlyHashedClientIP(t *testing.T) {
	key := submissionRateKey("203.0.113.9:54000")

	if !strings.HasPrefix(key, "submissions:create:") {
		t.Fatalf("key = %q", key)
	}
	if strings.Contains(key, "203.0.113.9") || strings.Contains(key, "author") || strings.Contains(key, "telegram") {
		t.Fatalf("key exposes input: %q", key)
	}
	if key != submissionRateKey("203.0.113.9:1000") {
		t.Fatalf("key depends on client port: %q", key)
	}
}
