package main

import (
	"sync/atomic"
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
	// emit asks the frontend whether quitting is safe.
	emit func()
	// quit really terminates the application.
	quit func()
}

// shouldQuit is called for every quit attempt (window close, Cmd/Ctrl+Q,
// menu quit). It allows quitting only after ConfirmQuit; otherwise it asks
// the frontend and refuses.
func (s *AppService) shouldQuit() bool {
	if s.allowQuit.Load() {
		return true
	}
	s.emit()
	return false
}

// ConfirmQuit is called by the frontend once there is nothing left to lose
// (or the user chose to discard it). It quits the application.
func (s *AppService) ConfirmQuit() {
	s.allowQuit.Store(true)
	s.quit()
}
