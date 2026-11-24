//go:build !ignore
// +build !ignore

// Copyright 2026 The Authors. All rights reserved.
// Use of this source code is governed by the MIT license.

// Compile-first mechanics adapted from itos-container T-10, f6f339c (AGPL-3.0).
// Ownership/linked graph probes follow itos-cc's architecture-check design.
package leave

import _ "unsafe"

//go:noinline
func Directive() {}

//go:generate echo generated

//line leave.go:100
func Line() {}

//nolint:gosec // the argument is checked at its edge
func Nolint() {}

//nolint:gosec,revive // both name the same reason
func Several() {}

func Nothing() {}

// itos-cc mutation: 1 killed, 1 survived, 0 uncovered
// survived: line 1 `>` → `>=` in Nothing
// end itos-cc mutation
