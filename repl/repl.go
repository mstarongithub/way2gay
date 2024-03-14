// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

type MessageHandler func(string, *Repl) (string, error)

// ReadCloser combines the Reader and Closer interfaces
type ReadCloser interface {
	io.Reader
	io.Closer
}

type Repl struct {
	Input   ReadCloser
	Output  io.WriteCloser
	scanner *bufio.Scanner
	writer  *bufio.Writer
}

// Creates a new repl
// If no input is given, stdin will be used
// If no output is given, stdout will be used
// Note: The given reader and writer will be closed if the repl is started and then stops
func NewRepl(in ReadCloser, out io.WriteCloser) Repl {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return Repl{
		Input:   in,
		Output:  out,
		scanner: bufio.NewScanner(in),
		writer:  bufio.NewWriter(out),
	}
}

// Starts the repl
// Blocks execution until the repl closes
// All input will be passed to the handler func
// If it receives an error from the message handler or during writing, it calls Close
func (r *Repl) Run(onMessage MessageHandler) error {
	for r.scanner.Scan() {
		newMessage := r.scanner.Text()
		res, err := onMessage(newMessage, r)
		if err != nil {
			r.Close()
			return fmt.Errorf("message handler errored out on message \"%s\": %w", newMessage, err)
		}
		if _, err = r.writer.WriteString(res + "\n"); err != nil {
			r.Close()
			return fmt.Errorf("failed to write result \"%s\": %w", res, err)
		}
		if err = r.writer.Flush(); err != nil {
			r.Close()
			return fmt.Errorf("failed to flush writer: %w", err)
		}
	}
	return nil
}

// Close stops the repl if it was still running
// This will also close the reader and writer
func (r *Repl) Close() {
	r.Input.Close()
	r.Output.Close()
}
