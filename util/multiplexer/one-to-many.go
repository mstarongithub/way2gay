// Copyright (c) 2024 mStar
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multiplexer

import (
	"errors"
	"sync"
)

type OneToMany[T any] struct {
	inbound   chan T
	outbound  map[string]chan T // Use map here to give names to outbound channels
	lock      sync.Mutex
	closeChan chan any
	closed    bool
}

func NewOneToMany[T any]() OneToMany[T] {
	return OneToMany[T]{
		inbound:   make(chan T),
		outbound:  make(map[string]chan T),
		lock:      sync.Mutex{},
		closeChan: make(chan any),
		closed:    false,
	}
}

// Get the channel to send things into
func (o *OneToMany[T]) GetSender() chan T {
	return o.inbound
}

// Create a new receiver for the multiplexer to send messages to.
// Please do not close this manually, instead use the CloseReceiver func
func (o *OneToMany[T]) MakeReceiver(name string) (chan T, error) {
	if o.closed {
		return nil, errors.New("multiplexer has been closed")
	}
	rec := make(chan T)

	// Only allow new receivers to be made
	o.lock.Lock()
	if _, ok := o.outbound[name]; ok {
		return nil, errors.New("receiver with that name already exists")
	}
	o.lock.Unlock()

	o.outbound[name] = rec

	return rec, nil
}

// Closes a receiver channel with the given name and removes it from the multiplexer
func (o *OneToMany[T]) CloseReceiver(name string) {
	if o.closed {
		return
	}
	o.lock.Lock()
	if val, ok := o.outbound[name]; ok {
		close(val)
		delete(o.outbound, name)
	}
	o.lock.Unlock()
}

// Start this one to many multiplexer
// intended to run as a goroutine (`go plexer.StartPlexer()`)
func (o *OneToMany[T]) StartPlexer() {
	select {
	// Message gotten from inbound channel
	case msg := <-o.inbound:
		o.lock.Lock()
		// Send it to all outbound channels
		for _, c := range o.outbound {
			c <- msg
		}
		o.lock.Unlock()
	// Told to close the plexer including sender
	case <-o.closeChan:
		o.lock.Lock()
		// First close all outbound channels
		// No need to send any signal there as readers will just stop
		for _, c := range o.outbound {
			close(c)
		}
		// Then close inbound, set closed and call it a day
		close(o.inbound)
		o.closed = true
		o.lock.Unlock()
		return
	}
}

// Close the sender and all receiver channels, mark the plexer as closed and stop the distribution goroutine (all by sending one signal)
func (o *OneToMany[T]) CloseSender() {
	o.closeChan <- 1
}
