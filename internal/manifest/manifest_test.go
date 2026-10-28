package manifest_test

import (
	"strings"
	"testing"

	"icarosu/internal/manifest"
)

const valid = `api_version: 1
id: tool.time
version: 0.1.0
kind: tool
name: 时间
provides: [tool.time]
requires: [kernel.tools]
tools:
  - name: time_now
    summary: 当前时间
`

func TestParseValid(t *testing.T) {
	m, err := manifest.Parse([]byte(valid))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.ID != "tool.time" || len(m.Tools) != 1 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func lines(parts ...string) string { return strings.Join(parts, "\n") }

func TestParseRejectsInvalid(t *testing.T) {
	cases := []string{
		lines("api_version: 2", "id: tool.time", "version: 0.1.0", "kind: tool", "provides: [tool.time]"),
		lines("api_version: 1", "id: time", "version: 0.1.0", "kind: tool", "provides: [tool.time]"),
		lines("api_version: 1", "id: tool.time", "version: nope", "kind: tool", "provides: [tool.time]"),
		lines("api_version: 1", "id: tool.time", "version: 0.1.0", "kind: tool", "provides: []"),
	}
	for _, c := range cases {
		if _, err := manifest.Parse([]byte(c)); err == nil {
			t.Fatalf("expected error for manifest:\n%s", c)
		}
	}
}
