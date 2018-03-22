// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package devdrawdriver

import (
	"image"
	"image/color"
)

type textureId uint32

type textureImpl struct {
	*uploadImpl
	size image.Point
}

func (t *textureImpl) Bounds() image.Rectangle {
	if t == nil {
		return image.ZR
	}
	return image.Rectangle{image.ZP, t.size}
}
func (t *textureImpl) Size() image.Point {
	if t == nil {
		return image.ZP
	}
	return t.size
}
func newTextureImpl(s *screenImpl, size image.Point) *textureImpl {
	uploader := newUploadImpl(s, image.Rectangle{image.ZP, size}, color.RGBA{0, 0, 0, 0})
	t := &textureImpl{
		uploadImpl: uploader,
		size:       size,
	}
	return t
}
