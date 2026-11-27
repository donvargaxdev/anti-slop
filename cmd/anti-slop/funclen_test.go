package main

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/kong"

	"github.com/donvargaxdev/anti-slop/internal/cli"
	"github.com/donvargaxdev/anti-slop/internal/engine/checking/port"
)

func functionLengthClassifier(t *testing.T) cli.Classifier {
	t.Helper()
	parser, err := kong.New(&commandLine{})
	if err != nil {
		t.Fatalf("build the command model: %v", err)
	}
	for _, node := range parser.Model.Children {
		if node.Name != "funclen" {
			continue
		}
		command, ok := node.Target.Interface().(interface {
			Classify(error) ([]*cli.Problem, bool)
		})
		if !ok {
			t.Fatal("funclen has no checker failure classifier")
		}
		return command.Classify
	}
	t.Fatal("funclen is missing from the command model")
	return nil
}

func TestClassifyDeclinesUnrelatedErrors(t *testing.T) {
	classify := functionLengthClassifier(t)
	if problems, ok := classify(errors.New("unrelated")); ok || problems != nil {
		t.Errorf("Classify(unrelated) = %v, %v, want nil, false", problems, ok)
	}
}

// Embedding lets the test exercise the fallback for an unknown checker failure.
type unexpectedFailure struct{ *port.Failed }

func TestClassifyCheckerFailures(t *testing.T) {
	classify := functionLengthClassifier(t)
	for _, tc := range []struct {
		name    string
		err     error
		code    int
		rule    string
		message string
	}{
		{"load error", &port.Failed{Reason: "invalid source"}, cli.CodeEnvironment,
			"funclen.check-failed", "the packages cannot be loaded: invalid source"},
		{"cancelled", &port.Cancelled{}, cli.CodeInterrupted,
			"interrupted", "interrupted before it finished"},
		{"unexpected failure", fmt.Errorf("wrapped: %w", unexpectedFailure{&port.Failed{Reason: "unexpected"}}),
			cli.CodeInternal, "internal", "cannot check: unexpected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems, ok := classify(tc.err)
			if !ok || len(problems) != 1 {
				t.Fatalf("Classify(%v) = %v, %v, want one problem and true", tc.err, problems, ok)
			}
			got := problems[0]
			if got.Code != tc.code || got.Rule != tc.rule || got.Message != tc.message {
				t.Errorf("problem = %+v, want code %d, rule %q and message %q", got, tc.code, tc.rule, tc.message)
			}
		})
	}
}

func TestFunctionLength(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	const fixtures = "internal/rules/funclen/testdata/src/"
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"catch", []string{
			"catch.go:3:1: funclen: function body is 71 lines; limit is 70",
			"catch.go:77:1: funclen: function body is 72 lines; limit is 70",
			"catch.go:154:16: funclen: function body is 71 lines; limit is 70",
			"ordinary_test.go:5:1: funclen: function body is 95 lines; limit is 70",
		}},
		{"leave", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"funclen", "./" + fixtures + tc.name + "/..."}
			code := run(t.Context(), args, strings.NewReader(""), &stdout, &stderr, diskAdapters())
			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("funclen %s: want exit 0 and empty stderr, got %d, stderr %q", tc.name, code, stderr.String())
			}
			var want strings.Builder
			for _, line := range tc.want {
				want.WriteString(filepath.FromSlash(fixtures+tc.name+"/") + line + "\n")
			}
			if stdout.String() != want.String() {
				t.Errorf("stdout = %q, want %q", stdout.String(), want.String())
			}
		})
	}
}
