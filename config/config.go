// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

type StartType int

const (
	// Tells way2gay to start a repl in parallel for interacting with it
	START_REPL = StartType(iota)
	// Tells way2gay to execute a specific command on startup
	START_SINGLE_COMMAND
	// Tells way2gay to start without any specific targets
	// Note: Good luck interacting with it :3
	START_NONE
)

type Config struct {
	StartType StartType `envconfig:"START_TYPE,omitempty" toml:"start_type,omitempty"`
	// What command to execute on start. Only matters if StartType is set to START_SINGLE_COMMAND
	StartCommand *string `envconfig:"START_COMMAND,omitempty" toml:"start_command,omitempty"`
}
