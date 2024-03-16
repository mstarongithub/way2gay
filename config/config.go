// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

type ConfigCommand struct {
	BaseKey    string   // Main key, for example Meta
	ModKeys    []string // Modifier keys such as Shift, Control, etc
	ActionKeys []string // Key to trigger that command
	Action     string   // Command to run
}

type ConfigScreen struct {
	Resolution  string  // Resolution the screen will run at (format is "<width>x<height>") (Resolution before Scaler is applied)
	RefreshRate int     // The refresh rate of the screen
	Position    string  // Where the screen is positioned in the global space (format is "<x>,<y>")
	Scaler      float32 // Fractional scaling factor (applied after resolution)
	// TODO: Check if wlroots dictates the order here in any way
}

type Config struct {
	// Screen config
	Screens map[string]string `json:"screens" toml:"screens" yaml:"screens"` // Per screen config. Missing screens will use their preferred mode

	// Commands
	Commands     map[string]ConfigCommand `json:"commands" toml:"commands" yaml:"commands"`                   // All the commands
	ExecOnStart  []string                 `json:"exec_on_start" toml:"exec_on_start" yaml:"exec_on_start"`    // Will be run once on start, not on config reload
	ExecOnReload []string                 `json:"exec_on_reload" toml:"exec_on_reload" yaml:"exec_on_reload"` // Will be run every time the config is reloaded
}
