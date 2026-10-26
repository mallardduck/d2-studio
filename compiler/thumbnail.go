package compiler

import (
	"context"
	"fmt"

	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/util-go/go2"
)

// RenderSVG compiles source with default options and returns its SVG. Like
// CompileForExport it takes no part in stale-result tracking, so rendering a
// gallery thumbnail never cancels, or is cancelled by, the live preview. The
// source is trusted app content (not a user's file): imports are not
// available and no images are read or fetched.
func (s *Service) RenderSVG(ctx context.Context, source string) (svg string, diags []Diagnostic, err error) {
	ctx = quietContext(ctx)
	s.run.Lock()
	defer s.run.Unlock()
	defer func() {
		if r := recover(); r != nil {
			svg, diags, err = "", internalError(r), nil
		}
	}()

	ropts := &d2svg.RenderOpts{NoXMLTag: go2.Pointer(true), OmitVersion: go2.Pointer(true)}
	c, cleanup, diags, isStale := s.compileDiagram(ctx, Request{Source: source}, ropts)
	if isStale {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		return "", nil, context.Canceled
	}
	if diags != nil {
		return "", diags, nil
	}
	defer cleanup()
	out, rerr := d2svg.Render(c.diagram, ropts)
	if rerr != nil {
		return "", nil, fmt.Errorf("render: %w", rerr)
	}
	return string(out), nil, nil
}
