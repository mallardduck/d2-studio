package main

import (
	"d2studio/export"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// wailsPlatform implements files.Platform with native Wails dialogs.
type wailsPlatform struct {
	app *application.App
}

func (p *wailsPlatform) PickOpenFile(dir string) (string, error) {
	d := p.app.Dialog.OpenFile().
		SetTitle("Open D2 diagram").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AddFilter("D2 diagrams (*.d2)", "*.d2").
		AddFilter("All files", "*")
	if w := p.app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	if dir != "" {
		d.SetDirectory(dir)
	}
	return d.PromptForSingleSelection()
}

func (p *wailsPlatform) PickSaveFile(dir, filename string) (string, error) {
	d := p.app.Dialog.SaveFile().
		SetMessage("Save D2 diagram").
		CanCreateDirectories(true).
		AddFilter("D2 diagrams (*.d2)", "*.d2").
		SetFilename(filename)
	if w := p.app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	if dir != "" {
		d.SetDirectory(dir)
	}
	return d.PromptForSingleSelection()
}

func (p *wailsPlatform) Emit(event string, data any) {
	p.app.Event.Emit(event, data)
}

// PickFolder implements workspace.Platform with a native folder dialog.
func (p *wailsPlatform) PickFolder(dir string) (string, error) {
	d := p.app.Dialog.OpenFile().
		SetTitle("Open folder").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true)
	if w := p.app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	if dir != "" {
		d.SetDirectory(dir)
	}
	return d.PromptForSingleSelection()
}

// PickExportFile implements export.Platform with a native save dialog that
// offers only the chosen format.
func (p *wailsPlatform) PickExportFile(dir, filename string, format export.Format) (string, error) {
	labels := map[export.Format]string{
		export.SVG: "SVG image (*.svg)",
		export.PNG: "PNG image (*.png)",
		export.PDF: "PDF document (*.pdf)",
	}
	d := p.app.Dialog.SaveFile().
		SetMessage("Export diagram").
		CanCreateDirectories(true).
		AddFilter(labels[format], "*."+format.Extension()).
		SetFilename(filename)
	if w := p.app.Window.Current(); w != nil {
		d.AttachToWindow(w)
	}
	if dir != "" {
		d.SetDirectory(dir)
	}
	return d.PromptForSingleSelection()
}
