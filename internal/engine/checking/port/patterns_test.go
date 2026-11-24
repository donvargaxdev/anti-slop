package port_test

import (
	"slices"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

func TestPatternsDistinguishLoaderQueriesFromPaths(t *testing.T) {
	for _, query := range []string{"file=p.go", "NAME=p", "Az=p", "Za=p"} {
		if _, err := port.NewPatterns([]string{query}); err == nil || err.Pattern != query {
			t.Errorf("NewPatterns(%q) refusal = %v, want the rejected query", query, err)
		}
	}
	paths := []string{".", "./...", "./file=p.go", "=p", "@=p", "[=p", "`=p", "{=p", "é=p"}
	got, err := port.NewPatterns(paths)
	if err != nil || !slices.Equal(got.List(), paths) {
		t.Errorf("NewPatterns(%q) = %v, %v, want all paths accepted", paths, got.List(), err)
	}
}
