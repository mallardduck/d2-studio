package main

import (
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventCloseRequested is emitted when the user tries to close the window or
// quit. The frontend decides whether it is safe (for example by asking about
// unsaved changes) and then calls ConfirmQuit.
const EventCloseRequested = "app:close-requested"

// AppService lets the frontend gate quitting. Closing is always cancelled
// first and the frontend is asked, so unsaved work is never discarded by the
// shell without the user's say-so.
type AppService struct {
	allowQuit atomic.Bool
	quit      func()
}

func NewAppService(quit func()) *AppService { return &AppService{quit: quit} }

// ShouldQuit is wired to the application and window close hooks: it allows
// quitting only after ConfirmQuit, and otherwise asks the frontend.
func (s *AppService) shouldQuit(app *application.App) bool {
	if s.allowQuit.Load() {
		return true
	}
	app.Event.Emit(EventCloseRequested)
	return false
}

// ConfirmQuit is called by the frontend once there is nothing left to lose
// (or the user chose to discard it). It quits the application.
func (s *AppService) ConfirmQuit() {
	s.allowQuit.Store(true)
	s.quit()
}
