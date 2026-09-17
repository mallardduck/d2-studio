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

func NewCompilerService() (*CompilerService, error) {
	svc, err := compiler.New(nil)
	if err != nil {
		return nil, err
	}
	return &CompilerService{svc: svc}, nil
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
