package tools

import (
	"strings"
	"testing"
	"time"
)

func TestVerdictAge(t *testing.T) {
	stale := time.Now().AddDate(0, -8, 0).Format("2006-01-02")
	if a := verdictAge(stale); !strings.Contains(a, "stale") {
		t.Errorf("8-month-old verdict should be tagged stale, got %q", a)
	}
	fresh := time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	if a := verdictAge(fresh); strings.Contains(a, "stale") {
		t.Errorf("1-month-old verdict should not be tagged stale, got %q", a)
	}
	if a := verdictAge("not-a-date"); a != "" {
		t.Errorf("unparseable date should return empty, got %q", a)
	}
}
