// Package export renders a compiled D2 diagram to SVG, PNG or PDF.
//
// PNG and PDF use D2's own pure-Go rasterizer, with the same limits as the
// `d2` command, so they work in a packaged app with no browser, no external
// converter and no network. The PDF embeds the rasterized diagram on a page
// (as D2's own PDF export does), so its text is not selectable; use SVG for a
// vector result.
package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/d2lang/d2/d2renderers/d2fonts"
	"github.com/d2lang/d2/d2renderers/d2raster"
	"github.com/d2lang/d2/d2renderers/d2scenebuild"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/d2target"
	"github.com/d2lang/d2/d2themes/d2themescatalog"
	"github.com/d2lang/d2/lib/imageasset"
	"github.com/d2lang/d2/lib/pdf"

	"d2studio/assets"
	"d2studio/compiler"
)

// errOffline is returned for every network request made while remote images
// are not allowed.
var errOffline = errors.New("remote images are disabled for this export")

// offlineTransport refuses all requests, so no connection is ever attempted.
type offlineTransport struct{}

func (offlineTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, errOffline }

// Format is an export file format.
type Format string

const (
	SVG Format = "svg"
	PNG Format = "png"
	PDF Format = "pdf"
)

// Extension is the file extension for the format, without the dot.
func (f Format) Extension() string { return string(f) }

// Options control an export.
type Options struct {
	// AllowRemoteImages lets PNG and PDF export download http(s) images such
	// as icons. It is off by default so that exporting works offline and never
	// makes a network request the user did not ask for.
	AllowRemoteImages bool
}

// Output is a rendered export.
type Output struct {
	Data []byte `json:"-"`
	// RemoteImages is how many distinct remote images the diagram uses.
	RemoteImages int `json:"remoteImages"`
	// RemoteImagesBlocked is true when those images were replaced by a
	// placeholder because AllowRemoteImages was off. SVG keeps the links, so
	// it is never "blocked".
	RemoteImagesBlocked bool `json:"remoteImagesBlocked"`
}

// Render renders the diagram in the given format.
func Render(ctx context.Context, f Format, in *compiler.ExportInput, opts Options) (out *Output, err error) {
	if in == nil || in.Diagram == nil {
		return nil, fmt.Errorf("nothing to export")
	}
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("export failed unexpectedly: %v", r)
		}
	}()

	switch f {
	case SVG:
		return renderSVG(in)
	case PNG, PDF:
		return renderRaster(ctx, f, in, opts)
	default:
		return nil, fmt.Errorf("unknown export format %q", f)
	}
}

func renderSVG(in *compiler.ExportInput) (*Output, error) {
	ro := in.Render
	// A standalone file wants its XML declaration, unlike the inline preview.
	ro.NoXMLTag = nil
	data, err := d2svg.Render(in.Diagram, &ro)
	if err != nil {
		return nil, fmt.Errorf("render SVG: %w", err)
	}
	p, err := countRemote(in)
	if err != nil {
		return nil, err
	}
	return &Output{Data: data, RemoteImages: p}, nil
}

// countRemote reports how many remote images a diagram uses without reading
// any local ones.
func countRemote(in *compiler.ExportInput) (int, error) {
	seen := map[string]struct{}{}
	add := func(u interface{ String() string }, scheme string) {
		if scheme == "http" || scheme == "https" {
			seen[u.String()] = struct{}{}
		}
	}
	for _, s := range append([]d2target.Shape{in.Diagram.Root}, in.Diagram.Shapes...) {
		if s.Icon != nil {
			add(s.Icon, s.Icon.Scheme)
		}
	}
	for _, c := range in.Diagram.Connections {
		if c.Icon != nil {
			add(c.Icon, c.Icon.Scheme)
		}
	}
	return len(seen), nil
}

func renderRaster(ctx context.Context, f Format, in *compiler.ExportInput, opts Options) (*Output, error) {
	prep, err := assets.Prepare(in.Diagram, in.ImportRoot, in.FileDir)
	if err != nil {
		return nil, err
	}

	pngData, err := rasterize(ctx, prep.Diagram, in.Render, opts.AllowRemoteImages)
	if err != nil {
		return nil, err
	}

	out := &Output{RemoteImages: len(prep.Remote), RemoteImagesBlocked: len(prep.Remote) > 0 && !opts.AllowRemoteImages}
	if f == PNG {
		out.Data = pngData
		return out, nil
	}

	data, err := toPDF(pngData, prep.Diagram, in.Render)
	if err != nil {
		return nil, err
	}
	out.Data = data
	return out, nil
}

// rasterize builds a scene for the diagram and paints it to PNG bytes.
func rasterize(ctx context.Context, diagram *d2target.Diagram, ro d2svg.RenderOpts, allowRemote bool) ([]byte, error) {
	var client *http.Client
	if !allowRemote {
		client = &http.Client{Transport: offlineTransport{}}
	}
	resolver, err := imageasset.New(imageasset.Options{HTTPClient: client, Limits: imageLimits()})
	if err != nil {
		return nil, fmt.Errorf("initialize image loading: %w", err)
	}
	fonts, err := newFontFallbackOptions()
	if err != nil {
		return nil, err
	}

	// Child boards are exported separately by D2; this exports the root board.
	board := *diagram
	board.Layers, board.Scenarios, board.Steps = nil, nil, nil

	sketch := ro.Sketch != nil && *ro.Sketch
	doc, err := d2scenebuild.Build(ctx, &board, d2scenebuild.Options{
		Pad:             ro.Pad,
		Scale:           ro.Scale,
		Center:          ro.Center,
		ThemeID:         ro.ThemeID,
		ThemeOverrides:  ro.ThemeOverrides,
		MaxNodes:        rasterMaxNodes,
		MaxPathCommands: rasterMaxPathCommands,
		Sketch:          sketch,
		SketchBudget: d2scenebuild.SketchBudget{
			MaxOperationSets: rasterMaxNodes,
			MaxOperations:    rasterMaxPathCommands,
			MaxPathCommands:  rasterMaxPathCommands,
		},
		Assets: &d2scenebuild.AssetOptions{
			Resolver:        resolver,
			SVGImportLimits: svgImportLimits(),
			SVGImportBudget: svgImportBudget(),
		},
		Fonts: fonts,
		LinkBudget: d2scenebuild.LinkBudget{
			MaxRegions: rasterMaxLinkRegions, MaxStringBytes: rasterMaxLinkStringBytes,
		},
		Appendix: true,
	})
	if err != nil {
		return nil, fmt.Errorf("build image: %w", err)
	}
	frame, err := d2raster.Render(ctx, doc, frameOptions())
	if err != nil {
		return nil, fmt.Errorf("render image: %w", err)
	}
	data, err := d2raster.EncodePNG(ctx, frame)
	if err != nil {
		return nil, fmt.Errorf("encode PNG: %w", err)
	}
	return data, nil
}

func newFontFallbackOptions() (*d2scenebuild.FontFallbackOptions, error) {
	system, err := d2fonts.NewSystemFallbackResolver(d2fonts.SystemFallbackLimits{
		MaxDirectoryEntries: fontSearchDirectoryEntries,
		MaxFiles:            fontSearchFiles,
		MaxFaces:            fontSearchFaces,
		MaxRequestedRunes:   fontSearchRequestedRunes,
		MaxCoverageChecks:   fontSearchCoverageChecks,
		MaxFileBytes:        fontSearchFileBytes,
		MaxScannedBytes:     fontSearchScannedBytes,
		MaxResolvedBytes:    fontSearchResolvedBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize font fallback: %w", err)
	}
	bundled, err := d2fonts.NewBundledFallbackResolver(system, d2fonts.BundledFallbackLimits{
		MaxRequestedRunes: fontSearchRequestedRunes,
		MaxBundledBytes:   fontBundledCopyBytes,
		MaxResolvedBytes:  fontBundledResolvedBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize bundled font fallback: %w", err)
	}
	return &d2scenebuild.FontFallbackOptions{
		Resolver: bundled, MaxAssets: fontAssetReserve, MaxBytes: fontAssetByteReserve,
		MaxRunesPerText:     fontMaxRunesPerText,
		MaxTotalRunes:       fontMaxTotalRunes,
		MaxCoverageChecks:   fontMaxCoverageChecks,
		MaxFontFacesPerText: fontAssetReserve,
		MaxShapingRuns:      rasterMaxTextShapingRuns,
		MaxShapedGlyphs:     rasterMaxPathCommands,
	}, nil
}

// toPDF places the rasterized diagram on a single PDF page.
func toPDF(pngData []byte, diagram *d2target.Diagram, ro d2svg.RenderOpts) ([]byte, error) {
	themeID := d2themescatalog.NeutralDefault.ID
	if ro.ThemeID != nil {
		themeID = *ro.ThemeID
	}
	var pad int64
	if ro.Pad != nil {
		pad = *ro.Pad
	}
	const boardID = "root"
	doc := pdf.Init()
	err := doc.AddPDFPage(pngData, []pdf.BoardTitle{{Name: diagram.Root.Label, BoardID: boardID}},
		themeID, diagram.Root.Fill, nil, pad, 0, 0, map[string]int{boardID: 0}, false)
	if err != nil {
		return nil, fmt.Errorf("build PDF: %w", err)
	}
	var buf bytes.Buffer
	if err := doc.ExportTo(&buf); err != nil {
		return nil, fmt.Errorf("write PDF: %w", err)
	}
	return buf.Bytes(), nil
}
