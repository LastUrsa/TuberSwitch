package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesToLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	logger, closeLog, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Print("hello log")
	closeLog()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "hello log") {
		t.Fatalf("log contents = %q", string(data))
	}
}
