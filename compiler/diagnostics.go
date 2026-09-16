package compiler

import (
	"context"
	"errors"
	"strings"

	"github.com/d2lang/d2/d2ast"
	"github.com/d2lang/d2/d2parser"
)

// diagnosticsFromError converts a D2 error into structured diagnostics.
// Parse and semantic errors from D2 are *d2parser.ParseError values carrying
// source ranges; any other error becomes a single diagnostic without a
// location rather than being dropped.
func diagnosticsFromError(err error, activePath string) []Diagnostic {
	var pe *d2parser.ParseError
	if errors.As(err, &pe) && !pe.Empty() {
		out := make([]Diagnostic, 0, len(pe.Errors))
		for _, e := range pe.Errors {
			out = append(out, fromASTError(e, activePath))
		}
		return out
	}
	return []Diagnostic{{Severity: SeverityError, Message: err.Error()}}
}

func fromASTError(e d2ast.Error, activePath string) Diagnostic {
	d := Diagnostic{
		Severity:    SeverityError,
		Message:     stripLocationPrefix(e.Message, e.Range),
		HasLocation: true,
		// D2 positions are zero-indexed.
		Line:      e.Range.Start.Line + 1,
		Column:    e.Range.Start.Column + 1,
		EndLine:   e.Range.End.Line + 1,
		EndColumn: e.Range.End.Column + 1,
		From:      e.Range.Start.Byte,
		To:        e.Range.End.Byte,
	}
	if e.Range.Path != "" && e.Range.Path != activePath {
		d.Path = e.Range.Path
	}
	if d.From < 0 {
		d.From, d.To = 0, 0
	}
	return d
}

// stripLocationPrefix removes the "path:line:col: " prefix D2 puts in
// messages, since the location is reported as structured fields.
func stripLocationPrefix(msg string, r d2ast.Range) string {
	prefix := r.String() + ": "
	if strings.HasPrefix(msg, prefix) {
		return msg[len(prefix):]
	}
	return msg
}

func isCancellation(err error) bool {
	return errors.Is(err, context.Canceled)
}
