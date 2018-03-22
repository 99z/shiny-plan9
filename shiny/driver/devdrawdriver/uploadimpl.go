// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"golang.org/x/exp/shiny/screen"
	"image"
	"image/color"
	"image/draw"
)

// uploadImpl implements the upload interface over /dev/draw
// and can be composed into anything that implements it for
// an image (notably windowImpl and textureImpl)
type uploadImpl struct {
	// writer to /dev/draw/n/data
	ctl *DrawCtrler
	// the imageId that represents this image in /dev/draw.
	imageId uint32
	// resources that were allocated which need to be
	// freed upon release.
	resources []uint32
}

func (u *uploadImpl) Release() {
	for _, id := range u.resources {
		u.ctl.FreeID(id)
	}
	u.ctl.FreeID(u.imageId)
}

func (u *uploadImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle) {
	img := src.RGBA()
	if img == nil {
		return
	}
	// get an image.RGBA referencing sr of Buffer.
	var subimage *image.RGBA = (img.SubImage(sr)).(*image.RGBA)

	// then replace the appropriate rectangle in this image.
	dr := image.Rectangle{
		Min: dp,
		Max: dp.Add(sr.Size()),
	}
	u.ctl.ReplaceSubimage(u.imageId, dr, subimage.Pix)
}

func (u *uploadImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {
	// create a new buffer with the appropriate colour and the appropriate
	// size.
	rect := image.Rectangle{image.ZP, dr.Size()}
	fillID := u.ctl.AllocBuffer(0, true, image.Rectangle{image.Point{0, 0}, image.Point{1, 1}}, rect, src)
	// we need a mask with the same shape, but a solid alpha channel.
	maskID := u.ctl.AllocBuffer(0, true, image.Rectangle{image.ZP, image.Point{1, 1}}, rect, color.Black)
	defer u.ctl.FreeID(maskID)
	defer u.ctl.FreeID(fillID)

	// then draw it on top of this image.
	u.ctl.Draw(uint32(u.imageId), fillID, maskID, dr, image.ZP, image.ZP, op)
}

func newUploadImpl(s *screenImpl, size image.Rectangle, c color.Color) *uploadImpl {
	// allocate a /dev/draw image id to represent this image.
	imageId := s.ctl.AllocBuffer(0, false, size, size, c)

	return &uploadImpl{
		ctl:       s.ctl,
		imageId:   imageId,
		resources: make([]uint32, 0),
	}
}
