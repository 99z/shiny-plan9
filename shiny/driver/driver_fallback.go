// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !darwin
// +build !linux android
// +build !windows
// +build !dragonfly
// +build !plan9

package driver

import (
	"errors"

	"github.com/niconan/shiny-plan9/shiny/driver/internal/errscreen"
	"github.com/niconan/shiny-plan9/shiny/screen"
)

func main(f func(screen.Screen)) {
	f(errscreen.Stub(errors.New("no driver for accessing a screen")))
}
