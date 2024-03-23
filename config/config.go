// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v3"
)

type (
	Config struct {
		// Screen config
		Screens map[string]ConfigScreen `json:"screens" toml:"screens" yaml:"screens"` // Per screen config. Missing screens will use their preferred mode. Key is screen name

		// Commands
		Commands map[string]ConfigCommand `json:"commands" toml:"commands" yaml:"commands"` // All the commands, key is command name
		OnStart  ConfigStartup            `json:"startup" toml:"startup" yaml:"startup"`    // Things to do on start
	}
	ConfigStartup struct {
		ExecOnStart  []string `json:"exec_on_start" toml:"exec_on_start" yaml:"exec_on_start"`    // Will be run once on start, not on config reload
		ExecOnReload []string `json:"exec_on_reload" toml:"exec_on_reload" yaml:"exec_on_reload"` // Will be run every time the config is reloaded
	}

	ConfigTiling struct {
		SplitToLeft bool `json:"split_left" toml:"split_left" yaml:"split_left"` // When splitting a leaf, should the original leaf be on the left or the right. True if left, false if right
	}
	ConfigScreen struct {
		Resolution  string  `json:"resolution" toml:"resolution" yaml:"resolution"`       // Resolution the screen will run at (format is "<width>x<height>") (Resolution before Scaler is applied)
		RefreshRate int     `json:"refresh_rate" toml:"refresh_rate" yaml:"refresh_rate"` // The refresh rate of the screen
		Position    string  `json:"position" toml:"position" yaml:"position"`             // Where the screen is positioned in the global space (format is "<x>,<y>")
		Scaler      float32 `json:"scale" toml:"scale" yaml:"scale"`                      // Fractional scaling factor (applied after resolution)
		// TODO: Check if wlroots dictates the order here in any way
	}
	ConfigCommand struct {
		BaseKey    string   `json:"base" toml:"base" yaml:"base"`                // Main key, for example Meta
		ModKeys    []string `json:"modifiers" toml:"modifiers" yaml:"modifiers"` // Modifier keys such as Shift, Control, etc
		ActionKeys []string `json:"keys" toml:"keys" yaml:"keys"`                // Key to trigger that command
		Action     string   `json:"action" toml:"action" yaml:"action"`          // Command to run
		// TODO: Make another struct for actions
		// One for executing an app
		// One for w2g actions
	}
)

// TODO: Fill in sane default values
var DEFAULT_CONFIG = Config{
	Screens:  map[string]ConfigScreen{},
	Commands: map[string]ConfigCommand{},
	OnStart:  ConfigStartup{},
}

// Parse a given config file
// Will fall back to first system default, then built-in default if the given file is not found or cannot be parsed
// Always returns a valid config, error will be set if it has to fall back
func ParseConfig(file string) (*Config, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		cfg, err := tryLoadSystemDefault()
		return cfg, fmt.Errorf("failed to open config file %s, falling back to system default. Error: %w", file, err)
	}
	fsplit := strings.Split(file, ".")
	if len(fsplit) > 1 {
		switch fsplit[len(fsplit)-1] {
		case "toml":
			cfg, err := loadWithUnmarshaller(content, toml.Unmarshal)
			return cfg, fmt.Errorf("failed to parse config file %s as toml, falling back to system default. Error: %w", file, err)
		// No extension, assume toml
		case "":
			cfg, err := loadWithUnmarshaller(content, toml.Unmarshal)
			return cfg, fmt.Errorf("failed to parse config file %s as toml, falling back to system default. Error: %w", file, err)
		case "json":
			cfg, err := loadWithUnmarshaller(content, json.Unmarshal)
			return cfg, fmt.Errorf("failed to parse config file %s as json, falling back to system default. Error: %w", file, err)
		case "yaml":

			cfg, err := loadWithUnmarshaller(content, yaml.Unmarshal)
			return cfg, fmt.Errorf("failed to parse config file %s as yaml, falling back to system default. Error: %w", file, err)
		case "yml":

			cfg, err := loadWithUnmarshaller(content, yaml.Unmarshal)
			return cfg, fmt.Errorf("failed to parse config file %s as yaml, falling back to system default. Error: %w", file, err)
		default:
			cfg, err := tryLoadSystemDefault()
			return cfg, fmt.Errorf("unknown file extension %s, falling back to system default. Error: %w", file, err)
		}
	} else {
		cfg, err := loadWithUnmarshaller(content, toml.Unmarshal)
		return cfg, fmt.Errorf("failed to parse config file %s as toml, falling back to system default. Error: %w", file, err)
	}
}

// Wrapper func to load config with a custom unmarshaller
// takes care of the fallback and error message thingy
func loadWithUnmarshaller(content []byte, unmarshal func([]byte, interface{}) error) (*Config, error) {
	config := Config{}
	err := unmarshal(content, &config)
	if err != nil {
		return tryLoadSystemDefault()
	}
	return &config, nil
}

// Try to load a system's default config for way2gay
// Will fall back to built-in default if it fails
// TODO: Check if /etc/way2gay/config.toml is the only place where default configs go
func tryLoadSystemDefault() (*Config, error) {
	content, err := os.ReadFile("/etc/way2gay/config.toml")
	if err != nil {
		return loadCompiledDefault(), fmt.Errorf("failed to read config /etc/way2gay/config.toml, falling back to built-in default. Error: %w", err)
	}
	config := Config{}
	err = toml.Unmarshal(content, &config)
	if err != nil {
		return loadCompiledDefault(), fmt.Errorf("failed to parse config /etc/way2gay/config.toml, falling back to built-in default. Error: %w", err)
	}
	return &config, nil
}

// Load the built-in default config
// Final fallback, should preferably NEVER be used
func loadCompiledDefault() *Config {
	return &DEFAULT_CONFIG
}

func getConfigDir() string {
	if xdg.ConfigHome != "" {
		return xdg.ConfigHome + "/way2gay"
	} else {
		return os.Getenv("HOME") + "/.config/way2gay"
	}
}
