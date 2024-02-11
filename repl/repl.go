// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package repl

import (
	"bufio"
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
	input   ReadCloser
	output  io.WriteCloser
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
		input:   in,
		output:  out,
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
		res, err := onMessage(r.scanner.Text(), r)
		if err != nil {
			r.Close()
			return err
		}
		if _, err = r.writer.WriteString(res + "\n"); err != nil {
			r.Close()
			return err
		}
		if err = r.writer.Flush(); err != nil {
			r.Close()
			return err
		}
	}
	return nil
}

// Close stops the repl if it was still running
// This will also close the reader and writer
func (r *Repl) Close() {
	r.input.Close()
	r.output.Close()
}