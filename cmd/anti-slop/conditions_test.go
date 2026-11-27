package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port/porttest"
)

func TestCompoundConditions(t *testing.T) {
	const reason = ": conditions: split this condition into named booleans\n"
	t.Chdir(filepath.Join("..", ".."))
	fixtures := filepath.Join("internal", "rules", "conditions", "testdata", "src")
	path := filepath.Join(fixtures, "catch", "catch.go")
	want := path + ":4:5" + reason +
		path + ":5:12" + reason +
		path + ":7:6" + reason +
		path + ":10:14" + reason +
		path + ":12:5" + reason +
		path + ":15:6" + reason

	for _, tc := range []struct {
		fixture string
		want    string
	}{
		{"catch", want},
		{"leave", ""},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			pattern := "./" + filepath.ToSlash(filepath.Join(fixtures, tc.fixture, "..."))
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), []string{"conditions", pattern}, strings.NewReader(""),
				&stdout, &stderr, diskAdapters())
			if code != 0 || stderr.Len() != 0 || stdout.String() != tc.want {
				t.Errorf("conditions %s exited %d, stdout %q, stderr %q; want success, stdout %q and no stderr",
					tc.fixture, code, stdout.String(), stderr.String(), tc.want)
			}
		})
	}
}

type unexpectedCheckFailure struct{ *port.Failed }

func TestConditionsClassifiesCheckerFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure port.CheckFailure
		code    int
		message string
	}{
		{"load error", &port.Failed{Reason: "invalid source"}, cli.CodeEnvironment,
			"the packages cannot be loaded: invalid source"},
		{"cancelled", &port.Cancelled{}, cli.CodeInterrupted, "interrupted before it finished"},
		{"unexpected failure", unexpectedCheckFailure{&port.Failed{Reason: "unexpected"}}, cli.CodeInternal,
			"cannot check: unexpected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checker := porttest.Checker{Failures: map[string]port.CheckFailure{"./...": tc.failure}}
			var stdout, stderr bytes.Buffer
			code := run(t.Context(), []string{"conditions", "./..."}, strings.NewReader(""),
				&stdout, &stderr, withFakeCheckers(adapters{}, checker))
			if code != tc.code || stdout.Len() != 0 || !strings.Contains(stderr.String(), "anti-slop: "+tc.message) {
				t.Errorf("conditions exited %d, stdout %q, stderr %q; want exit %d and error containing %q",
					code, stdout.String(), stderr.String(), tc.code, tc.message)
			}
		})
	}
}
