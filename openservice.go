package main

import "d2studio/openfile"

// EventOpenRequested is emitted when the operating system asks the app to open
// a file. It carries no data: the frontend calls OpenService.Take to collect
// the paths, so a request that arrives before the window has loaded is not lost.
const EventOpenRequested = "app:open-requested"

// OpenService hands the frontend the files the OS has asked it to open.
type OpenService struct {
	requests *openfile.Requests
}

// Take returns the paths waiting to be opened, oldest first, and clears them.
func (s *OpenService) Take() []string { return s.requests.Take() }
