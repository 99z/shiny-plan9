// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"log"
)

// Main spawns 2 goroutines to make blocking reads from /dev
// interfaces, one for the mouse and one for the keyboard.
// Window events such as resize and move come in over the mouse
// channel.
func Main(f func(s screen.Screen)) {
	mouseEvent := make(chan *mouse.Event)
	keyboardEvent := make(chan *key.Event)
	doneChan := make(chan bool)

	s, err := newScreenImpl()
	if err != nil {
		log.Fatalf("new screen: %v\n", err)
	}
	// read the current window size that will be drawn into from
	// /dev/wctl
	windowSize, err := readWctl()
	if err != nil {
		log.Fatalf("read current window size: %v\n", err)
	}

	s.windowFrame = windowSize

	go func() {
		// run the callback with the screen implementation, then send
		// a notification to break out of the infinite loop when it
		// exits
		f(s)
		doneChan <- true
		s.release()
	}()

	go mouseEventHandler(mouseEvent, s)
	go keyboardEventHandler(keyboardEvent)
	for {
		select {
		case mEv := <-mouseEvent:
			if s.w != nil {
				// translate the mouse event from the screen coordinate system to the window
				// coordinate system
				mEv.X -= float32(s.windowFrame.Min.X)
				mEv.Y -= float32(s.windowFrame.Min.Y)
				s.w.Deque.Send(*mEv)
			}
		case kEv := <-keyboardEvent:
			if s.w != nil {
				s.w.Deque.Send(*kEv)
			}
		case <-doneChan:
			return
		}
	}
}
