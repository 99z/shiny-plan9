// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"fmt"
	"image"
	"os"
	"strings"
)

// readWctl reads /dev/wctl to get the current Plan 9 window
// size. This is done once on startup to figure out the frame
// that will be used for drawing into, and after every resize
// event that comes from /dev/mouse to establish the new viewport.
func readWctl() (image.Rectangle, error) {
	ctl, err := os.OpenFile("/dev/wctl", os.O_RDWR, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current window status.\n")
		return image.ZR, err
	}
	defer ctl.Close()
	value := make([]byte, 1024) // 1024 should be enough..
	_, err = ctl.Read(value)
	if err != nil {
		return image.ZR, err
	}
	sizes := strings.Fields(string(value))
	// remove 4 pixels from each side to take rio's borders into consideration.
	return image.Rectangle{
		Min: image.Point{strToInt(sizes[0]) + 4, strToInt(sizes[1]) + 4},
		Max: image.Point{strToInt(sizes[2]) - 4, strToInt(sizes[3]) - 4},
	}, nil
}
