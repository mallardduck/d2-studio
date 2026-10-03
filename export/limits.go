package export

// These limits and settings mirror the ones the D2 command-line tool applies
// to PNG and PDF export (d2cli/raster.go in the pinned D2 version). Matching
// them keeps exports identical to `d2` and keeps the same safety ceilings on
// work done for any input.
//
// UPGRADING D2: re-read d2cli/raster.go and update this file to match. See
// docs/upgrading-d2.md.

import (
	"image/color"

	"github.com/d2lang/d2/d2renderers/d2raster"
	"github.com/d2lang/d2/d2renderers/d2scenebuild"
	"github.com/d2lang/d2/d2renderers/d2svgimport"
	"github.com/d2lang/d2/lib/imageasset"
)

const (
	rasterMaxDimension                = 32_768
	rasterMaxPixels             int64 = 64 * 1024 * 1024
	rasterMaxNodes                    = 1_000_000
	rasterMaxDepth                    = 1_024
	rasterMaxPathCommands             = 10_000_000
	rasterMaxTextRunesPerRun          = 100_000
	rasterMaxTextCoverageChecks int64 = 10_000_000
	rasterMaxTextShapingRuns          = 100_000
	rasterMaxAnimationTracks          = 1_000_000
	rasterMaxAnimationKeyframes       = 10_000_000
	rasterMaxOffscreenBytes     int64 = 512 * 1024 * 1024
	rasterMaxEvenOddClipWork    int64 = 250_000_000
	rasterMaxScanlineWork       int64 = 4_000_000_000

	rasterMaxAssets             = 4_096
	rasterMaxAssetBytes   int64 = 512 * 1024 * 1024
	rasterMaxDecodedBytes int64 = 512 * 1024 * 1024

	fontAssetReserve                 = 64
	fontAssetByteReserve       int64 = 64 * 1024 * 1024
	fontMaxRunesPerText              = rasterMaxTextRunesPerRun
	fontMaxTotalRunes                = rasterMaxPathCommands
	fontMaxCoverageChecks      int64 = 50_000_000
	fontSearchDirectoryEntries       = 20_000
	fontSearchFiles                  = 4_096
	fontSearchFaces                  = 8_192
	fontSearchRequestedRunes         = fontMaxRunesPerText
	fontSearchCoverageChecks         = 10_000_000
	fontSearchFileBytes        int64 = 32 * 1024 * 1024
	fontSearchScannedBytes     int64 = 256 * 1024 * 1024
	fontSearchResolvedBytes    int64 = 2 * fontAssetByteReserve
	fontBundledCopyBytes       int64 = 8 * 1024 * 1024
	fontBundledResolvedBytes         = fontBundledCopyBytes + fontSearchResolvedBytes

	imageAssetMaxBytes                  int64 = 32 * 1024 * 1024
	imageAssetMaxCount                        = rasterMaxAssets - fontAssetReserve
	imageAssetMaxCumulativeEncodedBytes int64 = rasterMaxAssetBytes - fontAssetByteReserve
	imageAssetMaxCumulativeDecodedBytes int64 = rasterMaxDecodedBytes

	rasterMaxLinkRegions     = 4_096
	rasterMaxLinkStringBytes = 1 * 1024 * 1024

	svgMaxBytes              = 64 * 1024
	svgMaxDepth              = 32
	svgMaxElements           = 256
	svgMaxAttributes         = 512
	svgMaxAttributeBytes     = 64 * 1024
	svgMaxPathCommands       = 4_096
	svgMaxTransformFunctions = 128
	svgMaxUseDepth           = 8
	svgMaxResources          = 64

	svgDocumentMaxSourceBytes          = 1 * 1024 * 1024
	svgDocumentMaxElements             = 4_096
	svgDocumentMaxAttributes           = 8_192
	svgDocumentMaxAttributeBytes       = 1 * 1024 * 1024
	svgDocumentMaxPathCommands         = 65_536
	svgDocumentMaxTransformFunctions   = 2_048
	svgDocumentMaxDeclaredResources    = 1_024
	svgDocumentMaxExpandedUseInstances = 1_024

	// rasterScale is D2's static raster device scale (lib/png.SCALE).
	rasterScale = 2
)

func frameOptions() d2raster.FrameOptions {
	return d2raster.FrameOptions{
		Scale:                 rasterScale,
		Background:            color.White,
		MaxWidth:              rasterMaxDimension,
		MaxHeight:             rasterMaxDimension,
		MaxPixels:             rasterMaxPixels,
		MaxNodes:              rasterMaxNodes,
		MaxDepth:              rasterMaxDepth,
		MaxPathCommands:       rasterMaxPathCommands,
		MaxTextRunesPerRun:    rasterMaxTextRunesPerRun,
		MaxFontFacesPerText:   fontAssetReserve,
		MaxTextCoverageChecks: rasterMaxTextCoverageChecks,
		MaxTextShapingRuns:    rasterMaxTextShapingRuns,
		MaxAnimationTracks:    rasterMaxAnimationTracks,
		MaxAnimationKeyframes: rasterMaxAnimationKeyframes,
		MaxAssets:             rasterMaxAssets,
		MaxAssetBytes:         rasterMaxAssetBytes,
		MaxDecodedAssetBytes:  rasterMaxDecodedBytes,
		MaxImportDepth:        rasterMaxDepth,
		MaxOffscreenBytes:     rasterMaxOffscreenBytes,
		MaxEvenOddClipWork:    rasterMaxEvenOddClipWork,
		MaxScanlineWork:       rasterMaxScanlineWork,
	}
}

func svgImportLimits() d2svgimport.Limits {
	return d2svgimport.Limits{
		MaxBytes:              svgMaxBytes,
		MaxDepth:              svgMaxDepth,
		MaxElements:           svgMaxElements,
		MaxAttributes:         svgMaxAttributes,
		MaxAttributeBytes:     svgMaxAttributeBytes,
		MaxPathCommands:       svgMaxPathCommands,
		MaxTransformFunctions: svgMaxTransformFunctions,
		MaxUseDepth:           svgMaxUseDepth,
		MaxResources:          svgMaxResources,
	}
}

func svgImportBudget() d2scenebuild.SVGImportBudget {
	return d2scenebuild.SVGImportBudget{
		MaxSourceBytes:          svgDocumentMaxSourceBytes,
		MaxElements:             svgDocumentMaxElements,
		MaxAttributes:           svgDocumentMaxAttributes,
		MaxAttributeBytes:       svgDocumentMaxAttributeBytes,
		MaxPathCommands:         svgDocumentMaxPathCommands,
		MaxTransformFunctions:   svgDocumentMaxTransformFunctions,
		MaxDeclaredResources:    svgDocumentMaxDeclaredResources,
		MaxExpandedUseInstances: svgDocumentMaxExpandedUseInstances,
	}
}

func imageLimits() imageasset.Limits {
	return imageasset.Limits{
		MaxFetchedBytes:           min(imageAssetMaxBytes, imageAssetMaxCumulativeEncodedBytes),
		MaxEncodedBytes:           min(imageAssetMaxBytes, imageAssetMaxCumulativeEncodedBytes),
		MaxDecompressedBytes:      min(imageAssetMaxBytes, imageAssetMaxCumulativeEncodedBytes),
		MaxSVGBytes:               svgMaxBytes,
		MaxDecodedWidth:           rasterMaxDimension,
		MaxDecodedHeight:          rasterMaxDimension,
		MaxDecodedPixels:          rasterMaxPixels,
		MaxAssets:                 imageAssetMaxCount,
		MaxCumulativeEncodedBytes: imageAssetMaxCumulativeEncodedBytes,
		MaxCumulativeDecodedBytes: imageAssetMaxCumulativeDecodedBytes,
	}
}
