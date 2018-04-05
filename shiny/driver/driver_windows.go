// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"github.com/niconan/shiny-plan9/shiny/driver/windriver"
	"github.com/niconan/shiny-plan9/shiny/screen"
)

func main(f func(screen.Screen)) {
	windriver.Main(f)
}
