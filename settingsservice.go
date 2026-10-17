package main

import (
	"errors"

	"d2studio/logging"
	"d2studio/settings"
)

var settingsLog = logging.For("settings")

// SettingsService exposes persisted user preferences to the frontend.
type SettingsService struct {
	store *settings.Store
}

// NewSettingsService stores settings in the OS config directory. If that
// directory cannot be determined the app still runs with defaults, and Save
// reports the problem instead of writing somewhere unexpected.
func NewSettingsService() *SettingsService {
	path, err := settings.DefaultPath()
	if err != nil {
		settingsLog.Warn("cannot locate the settings folder; preferences will not be saved", "err", err)
		return &SettingsService{}
	}
	return &SettingsService{store: settings.NewStore(path)}
}

// Get returns the saved settings, or defaults when none exist or the file is
// unreadable (the problem is logged; the user's file is not modified).
func (s *SettingsService) Get() settings.Settings {
	if s.store == nil {
		return settings.Defaults()
	}
	got, err := s.store.Load()
	if err != nil {
		settingsLog.Warn("could not read settings; using defaults", "err", err)
	}
	return got
}

// Save persists settings. Invalid values are normalized before writing.
func (s *SettingsService) Save(in settings.Settings) error {
	if s.store == nil {
		return errors.New("settings location is unavailable")
	}
	return s.store.Save(in)
}
