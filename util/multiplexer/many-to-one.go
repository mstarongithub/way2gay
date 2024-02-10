// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multiplexer

import "errors"

// A many to one multiplexer
// Yes, channels technically already are that, but there are a bunch of problems with using raw channels as multiplexer:
// If any of the senders tries to send to a closed channel, it explodes
// Thus, wrap it inside a struct that handles that case of a closed channel
type ManyToOne[T any] struct {
	outbound chan T
	closed   bool
}

// NewManyToOne creates a new ManyToOne multiplexer
// The given channel will be where all messages will be sent to
func NewManyToOne[T any](receiver chan T) ManyToOne[T] {
	return ManyToOne[T]{
		outbound: receiver,
		closed:   false,
	}
}

// Send a message to this many to one plexer
// If closed, the message won't get sent
func (m *ManyToOne[T]) Send(msg T) error {
	if m.closed {
		return errors.New("multiplexer has been closed")
	}
	m.outbound <- msg
	return nil
}

// Closes the channel and marks the plexer as closed
func (m *ManyToOne[T]) Close() {
	close(m.outbound)
	m.closed = true
}
