package main

import (
	"context"
	"fmt"
	"sync"

	"d2studio/compiler"
	"d2studio/snippets"
)

// SnippetService serves the built-in example gallery to the frontend.
type SnippetService struct {
	svc *compiler.Service

	mu    sync.Mutex
	cache map[string]string // snippet ID -> rendered SVG
}

func NewSnippetService(svc *compiler.Service) *SnippetService {
	return &SnippetService{svc: svc, cache: map[string]string{}}
}

// List returns the examples in display order.
func (s *SnippetService) List() []snippets.Snippet { return snippets.All() }

// Preview returns the SVG for one example. Results are cached, since the
// examples never change while the app runs.
func (s *SnippetService) Preview(ctx context.Context, id string) (string, error) {
	sn, ok := snippets.ByID(id)
	if !ok {
		return "", fmt.Errorf("unknown snippet %q", id)
	}
	s.mu.Lock()
	if svg, ok := s.cache[id]; ok {
		s.mu.Unlock()
		return svg, nil
	}
	s.mu.Unlock()

	svg, diags, err := s.svc.RenderSVG(ctx, sn.Code)
	if err != nil {
		return "", err
	}
	if len(diags) > 0 {
		return "", fmt.Errorf("snippet %q does not compile: %s", id, diags[0].Message)
	}
	s.mu.Lock()
	s.cache[id] = svg
	s.mu.Unlock()
	return svg, nil
}
