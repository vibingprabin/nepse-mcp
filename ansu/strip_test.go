package ansu

import (
	"strings"
	"testing"
)

func TestStripDisclaimer(t *testing.T) {
	with := "The analysis is here.\n\nImportant information: This content is for informational purposes only. Investors may receive less than their initial investment."
	got := StripDisclaimer(with)
	if strings.Contains(got, "Important information") {
		t.Errorf("StripDisclaimer left disclaimer: %q", got)
	}
	if !strings.Contains(got, "The analysis is here") {
		t.Errorf("StripDisclaimer removed content: %q", got)
	}

	clean := "Just analysis, no disclaimer."
	if StripDisclaimer(clean) != clean {
		t.Errorf("StripDisclaimer altered clean text: %q", StripDisclaimer(clean))
	}
}
