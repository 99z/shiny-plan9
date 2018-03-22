// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"image"
)

// Just use an in-memory RGBA image as a buffer. It'll
// get written to /dev/draw/n when it's uploaded to
// a texture
type bufferImpl struct {
	i *image.RGBA
}

func (b *bufferImpl) Release() {
	b.i = nil
	// the image will get garbage collected
}

func (b *bufferImpl) RGBA() *image.RGBA {
	return b.i
}

func (b *bufferImpl) Bounds() image.Rectangle {
	return b.i.Bounds()
}

func (b *bufferImpl) Size() image.Point {
	return b.i.Bounds().Size()
}
