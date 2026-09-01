package news

import (
	"strings"
	"testing"
)

const pmSample = `{"type":"doc","content":[
  {"type":"paragraph","content":[{"type":"text","text":"Revenue from operations declined 24.51% YoY."}]},
  {"type":"table","content":[
    {"type":"tableRow","content":[
      {"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"Particulars"}]}]},
      {"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"FY 2082/83"}]}]}
    ]},
    {"type":"tableRow","content":[
      {"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"Revenue"}]}]},
      {"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"Rs. 18.01 crore"}]}]}
    ]}
  ]},
  {"type":"paragraph","content":[{"type":"text","text":"Net loss of Rs. 5.33 crore."}]}
]}`

func TestExtractProseMirror(t *testing.T) {
	got := extractProseMirror(pmSample)
	for _, want := range []string{
		"Revenue from operations declined 24.51% YoY.",
		"Particulars | FY 2082/83", // table header row, pipe-joined
		"Revenue | Rs. 18.01 crore", // data row
		"Net loss of Rs. 5.33 crore.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("extractProseMirror missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "tableRow") || strings.Contains(got, `"type"`) {
		t.Errorf("extractProseMirror leaked JSON structure:\n%s", got)
	}
}

func TestExtractProseMirrorFallback(t *testing.T) {
	// Non-JSON content should pass through trimmed.
	if got := extractProseMirror("plain text body"); got != "plain text body" {
		t.Errorf("fallback = %q, want passthrough", got)
	}
}
