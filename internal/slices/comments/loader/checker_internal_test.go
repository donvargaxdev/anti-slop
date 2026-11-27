package loader

import (
	"context"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

func TestCancellationWhileReportingSources(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.go")
	if err := os.WriteFile(first, []byte("package p\n\n// unread\nvar n int\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		files []string
	}{
		{"before another parse", []string{first, filepath.Join(dir, "missing.go")}},
		{"after the last file", []string{first}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			failure := reportSources(ctx, token.NewFileSet(), tc.files, func(analysis.Diagnostic) {
				cancel()
			})
			if _, ok := failure.(*port.Cancelled); !ok {
				t.Errorf("reportSources = %v, want *port.Cancelled", failure)
			}
		})
	}
}
