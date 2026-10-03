package main

import (
	"context"

	"d2studio/compiler"
)

// CompilerService exposes the D2 compiler to the frontend. Wails generates
// TypeScript bindings for its exported methods.
type CompilerService struct {
	svc *compiler.Service
}

// NewCompilerService wraps svc, which is shared with the exporter so that
// previews and exports use one text-measuring ruler under one lock.
func NewCompilerService(svc *compiler.Service) *CompilerService {
	return &CompilerService{svc: svc}
}

// Compile compiles D2 source to SVG. D2 errors come back as diagnostics in
// the result, not as a Go error. A result with status "stale" must be
// discarded by the caller.
func (s *CompilerService) Compile(ctx context.Context, req compiler.Request) (*compiler.Result, error) {
	return s.svc.Compile(ctx, req)
}

// Engines lists the layout engines supported by this build.
func (s *CompilerService) Engines() []compiler.Engine { return compiler.Engines() }

// Themes lists the themes from D2's catalog.
func (s *CompilerService) Themes() []compiler.Theme { return compiler.Themes() }

// Language returns D2's keyword and value vocabulary for syntax highlighting.
func (s *CompilerService) Language() compiler.LanguageInfo { return compiler.Language() }

// Complete returns context-aware completions. line and column are zero-based
// and column counts UTF-16 code units.
func (s *CompilerService) Complete(source string, line, column int) []compiler.Completion {
	return compiler.Complete(source, line, column)
}
