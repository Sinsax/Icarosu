package logger_test

import (
	"testing"

	"icarosu/internal/logger"
)

func TestBufferKeepsRecentLines(t *testing.T) {
	buf := logger.NewBuffer(2)
	_, _ = buf.Write([]byte("line1\nline2\nline3\n"))

	lines := buf.Recent(10)
	if len(lines) != 2 || lines[0] != "line2" || lines[1] != "line3" {
		t.Fatalf("unexpected recent lines: %v", lines)
	}
}
