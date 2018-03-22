// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
)

var NoScreen error = errors.New("Could not allocate screen")

// A DrawCtrler is an object which holds references to
// /dev/draw/n/^(data ctl), and allows you to send or
// receive messages from it.
type DrawCtrler struct {
	N    int
	ctl  io.ReadWriteCloser
	data io.ReadWriteCloser

	// the maxmum message size that can be written to
	// /dev/draw/data.
	iounitSize int
	// the next available ID to use when allocating
	// an image
	nextId uint32

	// A mutex to avoid race conditions with Draw/SetOp
	drawMu sync.Mutex
}

// A DrawCtlMsg represents the data that is returned from
// opening /dev/draw/new or reading /dev/draw/n/ctl.
type DrawCtlMsg struct {
	N int

	DisplayImageId int
	ChannelFormat  string
	MysteryValue   string
	DisplaySize    image.Rectangle
	Clipping       image.Rectangle
}

const NewScreen = "/dev/draw/new"

// NewDrawCtrler creates a new DrawCtrler to interact with
// the /dev/draw filesystem. It returns a reference to
// a DrawCtrler, and a DrawCtlMsg representing the data
// that was returned from opening /dev/draw/new.
func NewDrawCtrler() (*DrawCtrler, *DrawCtlMsg, error) {
	fNew, err := os.Open(NewScreen)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not open %s: %v\n", NewScreen, err)
	}
	defer fNew.Close()

	// id 1 reserved for the image represented by /dev/winname, so
	// start allocating new IDs at 2.
	dc := &DrawCtrler{nextId: 2}
	ctlString := dc.readCtlString(fNew)
	msg := parseCtlString(ctlString)
	if msg == nil {
		return dc, nil, fmt.Errorf("Could not parse ctl string from %s: %s\n", NewScreen, ctlString)
	}

	if msg.N < 1 {
		// huh? what now?
		return nil, nil, fmt.Errorf("draw index less than one: %d", msg.N)
	}
	dc.N = msg.N
	//      open the data channel for the connection we just created so
	//      we can send messages to it.  We don't close it so that it
	//      doesn't disappear from the /dev filesystem on us.  It needs
	//      to be closed when the screen is cleaned up.
	fn := fmt.Sprintf("/dev/draw/%d/data", msg.N)
	fData, err := os.OpenFile(fn, os.O_RDWR, 0)
	if err != nil {
		return dc, msg, fmt.Errorf("Could not open %s: %v\n", fn, err)
	}
	dc.data = fData

	// read the iounit size from the /proc filesystem.
	pid := os.Getpid()
	if fdInfo, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/fd", pid)); err == nil {
		lines := bytes.Split(fdInfo, []byte{'\n'})
		// See man proc(3) for a description of the format of /proc/$pid/fd that's
		// being parsed to find the iounit size
		// the first line is just the current wd, so don't range over it
		for _, line := range lines[1:] {
			fInfo := bytes.Fields(line)
			if len(fInfo) >= 10 && string(fInfo[9]) == fn {
				// found /dev/draw/N/data in the list of open files, so get
				// the iounit size of it.
				i, err := strconv.Atoi(string(fInfo[7]))
				if err != nil {
					return nil, nil, fmt.Errorf("Invalid iounit size. Could not convert to integer.")
				}
				dc.iounitSize = i
				break

			}

		}

		if dc.iounitSize == 0 {
			return nil, nil, fmt.Errorf("Could not parse iounit size.\n")
		}
	} else {
		return nil, nil, fmt.Errorf("Could not determine iounit size: %v\n", err)
	}
	return dc, msg, nil
}

// reads the output of /dev/draw/new or /dev/draw/n/ctl and returns
// it without doing any parsing.  It should be passed along to
// parseCtlString to create a *DrawCtlMsg
func (d DrawCtrler) readCtlString(f io.Reader) string {
	val := make([]byte, 256)
	n, err := f.Read(val)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading control string: %s\n", err)
		return ""
	}
	// there are 12 11 character wide strings in a ctl message, each followed
	// by a space. The last one may or may not have a terminating space, depending
	// on draw implementation, but it's irrelevant if it does.
	if err != nil || n < 143 {
		fmt.Fprintf(os.Stderr, "Incorrect number of bytes in ctl string: %d\n", n)
		return ""
	}
	return string(val[:144])
}

// sendMessage sends the command represented by cmd to the data channel,
// with the raw arguments in val (n.b. They need to be in little endian
// byte order and match the cmd arguments described in draw(3))
func (d DrawCtrler) sendMessage(cmd byte, val []byte) error {
	realCmd := append([]byte{cmd}, val...)
	_, err := d.data.Write(realCmd)
	return err
}

// Sends a message to /dev/draw/n/ctl.
// This isn't used, but might be in the future.
func (d DrawCtrler) sendCtlMessage(val []byte) error {
	_, err := d.ctl.Write(val)
	return err
}

// Allocates a new screen and returns either the ID for
// the screen, or a NoScreen error.
func (d *DrawCtrler) AllocScreen() (screenId, error) {
	msg := make([]byte, 13)
	for i := 0; i < 255; i++ {
		binary.LittleEndian.PutUint32(msg[0:], uint32(i))
		err := d.sendMessage('A', msg)
		if err == nil {
			return screenId(i), nil
		}
	}
	return 0, NoScreen
}

// Frees the screen identified by id.
func (d *DrawCtrler) FreeScreen(id screenId) {
	msg := make([]byte, 4)
	binary.LittleEndian.PutUint32(msg, uint32(id))
	d.sendMessage('F', msg)
}

// Reallocate a screen.
func (d *DrawCtrler) ReallocScreen(id screenId) error {
	d.drawMu.Lock()
	defer d.drawMu.Unlock()

	// Free the screen!
	msg := make([]byte, 4)
	binary.LittleEndian.PutUint32(msg, uint32(id))
	d.sendMessage('F', msg)

	// Alloc the same screen!
	msg = make([]byte, 13)
	binary.LittleEndian.PutUint32(msg[0:], uint32(id))
	return d.sendMessage('A', msg)
}

// AllocBuffer will send a message to /dev/draw/N/data of the form:
//    b id[4] screenid[4] refresh[1] chan[4] repl[1] r[4*r] clipr[4*4] color[4]
// see draw(3) for details.
//
// For the purposes of the using this helper method, id and screenid are
// automatically generated by the DrawDriver, and chan is always an RGBA
// channel.
//
// Returns the ID that can be used to reference the allocated buffer
func (d *DrawCtrler) AllocBuffer(refresh byte, repl bool, r, clipr image.Rectangle, color color.Color) uint32 {
	msg := make([]byte, 50)
	// id is the next available ID.
	d.nextId += 1
	newId := d.nextId
	binary.LittleEndian.PutUint32(msg[0:], newId)
	// refresh can just be passed along directly.
	msg[8] = refresh

	// RGBA channel. This is the same format as image.RGBA.Pix,
	// so that we can directly upload a buffer.
	msg[9] = 8   // r8
	msg[10] = 24 // g8
	msg[11] = 40 // b8
	msg[12] = 72 // a8
	// Convert repl from bool to a byte
	if repl == true {
		msg[13] = 1
	}

	// Convert the rectangle to little endian in the appropriate
	// places for the message
	binary.LittleEndian.PutUint32(msg[14:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[18:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(msg[22:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(msg[26:], uint32(r.Max.Y))
	binary.LittleEndian.PutUint32(msg[30:], uint32(clipr.Min.X))
	binary.LittleEndian.PutUint32(msg[34:], uint32(clipr.Min.Y))
	binary.LittleEndian.PutUint32(msg[38:], uint32(clipr.Max.X))
	binary.LittleEndian.PutUint32(msg[42:], uint32(clipr.Max.Y))
	// RGBA colour to use by default for this buffer.
	// color.RGBA() returns a uint16 (actually a uint32
	// with only the lower 16 bits set), so shift it to
	// convert it to a uint8.

	// Note that there's a bug in libmemdraw in the standard Plan 9
	// distribution that the endianness is sometimes swapped, but
	// we don't do anything about it here because that would break
	// drawterm, 9front, or anything else where it's implemented
	// according to the spec..
	rd, g, b, a := color.RGBA()
	msg[46] = byte(a >> 8)
	msg[47] = byte(b >> 8)
	msg[48] = byte(g >> 8)
	msg[49] = byte(rd >> 8)

	d.sendMessage('b', msg)
	return newId
}

// FreeID will release the resources held by the imageID in this
// /dev/draw interface.
func (d *DrawCtrler) FreeID(id uint32) {
	// just convert to little endian and send the id to 'f'
	msg := make([]byte, 4)
	binary.LittleEndian.PutUint32(msg, id)
	d.sendMessage('f', msg)
}

// SetOp sets the compositing operation for the next draw to op.
//
// This isn't exposed, because it should only be called by Draw,
// which needs to apply a mutex.
func (d *DrawCtrler) setOp(op draw.Op) {
	// valid options according to draw(2):
	//	Clear = 0
	//	SinD  = 8
	//	DinS  = 4
	//	SoutD = 2
	//	DoutS = 1
	//	S     = SinD|SoutD (== 10)
	//	SoverD= SinD|SoutD|DoutS (==11)
	// etc.. but S and SoverD are the only valid
	// draw ops in Go
	msg := make([]byte, 1)
	switch op {
	case draw.Src:
		msg[0] = 10
	case draw.Over:
		fallthrough
	default:
		msg[0] = 11
	}
	d.sendMessage('O', msg)
}

// Draw formats the parameters appropriate to send the message:
//    d dstid[4] srcid[4] maskid[4] dstr[4*4] srcp[2*4] maskp[2*4]
// to /dev/draw/n/data.
// See draw(3) for details.
func (d *DrawCtrler) Draw(dstid, srcid, maskid uint32, r image.Rectangle, srcp, maskp image.Point, op draw.Op) {
	d.drawMu.Lock()
	defer d.drawMu.Unlock()

	d.setOp(op)

	msg := make([]byte, 44)
	binary.LittleEndian.PutUint32(msg[0:], dstid)
	binary.LittleEndian.PutUint32(msg[4:], srcid)
	binary.LittleEndian.PutUint32(msg[8:], maskid)
	binary.LittleEndian.PutUint32(msg[12:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[16:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(msg[20:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(msg[24:], uint32(r.Max.Y))
	binary.LittleEndian.PutUint32(msg[28:], uint32(srcp.X))
	binary.LittleEndian.PutUint32(msg[32:], uint32(srcp.Y))
	binary.LittleEndian.PutUint32(msg[36:], uint32(maskp.X))
	binary.LittleEndian.PutUint32(msg[40:], uint32(maskp.Y))
	d.sendMessage('d', msg)
}

// Implements the compression format described in image(6) for use in
// 'Y' messages if the /dev/draw driver isn't libmemdraw.
func (d *DrawCtrler) compressedReplaceSubimage(dstid uint32, r image.Rectangle, pixels []byte) {
	// "Pixels are encoding using a version of Lempel & Ziv's sliging window scheme LZ77."
	// We don't care about the rest of image(6), because we're not using the image format,
	// just the same LZ77 compression.

	// There's 4 bytes per pixel in an RGBA, so for each iteration compress
	// rSize.X*4 = 1 line of data, check if it's over the iounit size, and send
	// the Y message before appending it if so.

	blockYStart := 0
	rSize := r.Size()

	compressed := make([]byte, 0)
	// use rSize instead of r.Min.Y to make indexing into pixels easier.
	for i := 0; i < rSize.Y; i += 1 {

		rowStart := i * 4 * rSize.X
		linePixels := pixels[rowStart : rowStart+(rSize.X*4)]
		compressedLine := compress(linePixels)
		// Note that even though image(6) says the compression format should be less
		// than 6000 to fit in a 9p unit, we're actually just using the lz77 compression
		// described. We know the iounitSize, so use it as the cutoff.
		if len(compressed)+len(compressedLine) >= d.iounitSize || i == rSize.Y-1 {
			// construct the message for /dev/draw/data
			msg := make([]byte, 20+len(compressed))
			binary.LittleEndian.PutUint32(msg[0:], dstid)
			binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
			binary.LittleEndian.PutUint32(msg[8:], uint32(r.Min.Y+blockYStart))
			binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
			binary.LittleEndian.PutUint32(msg[16:], uint32(r.Min.Y+i))
			copy(msg[20:], compressed)
			d.sendMessage('Y', msg)

			// keep track of information for the next message
			blockYStart = i
			compressed = compressedLine
		} else {
			compressed = append(compressed, compressedLine...)
		}

	}
}

// ReplaceSubimage replaces the rectangle r with the pixel buffer
// defined by pixels.
//
// It sends /dev/draw/n/data the message:
//	y id[4] r[4*4] buf[x*1]
func (d *DrawCtrler) ReplaceSubimage(dstid uint32, r image.Rectangle, pixels []byte) {
	// 9p limits the reads and writes to the iounit size, which is read from /proc/$pid/fd
	// at startup. So we need to split up the command into multiple 'y' commands of the
	// maximum iounit size if it doesn't fit in 1 message.
	if d.iounitSize < 65535 && len(pixels) > 256 {
		// the in-memory /dev/draw driver has an iounit size of 65535. If it's less than
		// that, it's probably because it's a remote implementation with some overhead
		// somewhere.
		// In that case, use the compresssed 'Y' form instead and skip this.
		// Don't bother with small images, because the overhead of the compression will
		// probably be worse than the gain. 256 is entirely arbitrary.
		d.compressedReplaceSubimage(dstid, r, pixels)
		return
	}
	rSize := r.Size()
	if (rSize.X*rSize.Y*4 + 21) < d.iounitSize {
		msg := make([]byte, 20+(rSize.X*rSize.Y*4))
		binary.LittleEndian.PutUint32(msg[0:], dstid)
		binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
		binary.LittleEndian.PutUint32(msg[8:], uint32(r.Min.Y))
		binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
		binary.LittleEndian.PutUint32(msg[16:], uint32(r.Max.Y))

		copy(msg[20:], pixels)
		d.sendMessage('y', msg)
		return
	}

	lineSize := d.iounitSize / 4 / rSize.X
	msg := make([]byte, 20+(rSize.X*lineSize*4))
	binary.LittleEndian.PutUint32(msg[0:], dstid)
	binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
	for i := r.Min.Y; i < r.Max.Y; i += lineSize {
		endline := i + lineSize
		if endline > r.Max.Y {
			endline = r.Max.Y
			msg = make([]byte, 20+(rSize.X*(endline-i)*4))
			binary.LittleEndian.PutUint32(msg[0:], dstid)
			binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
			binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))

		}
		binary.LittleEndian.PutUint32(msg[8:], uint32(i))
		binary.LittleEndian.PutUint32(msg[16:], uint32(endline))
		copy(msg[20:], pixels[i*rSize.X*4:])
		d.sendMessage('y', msg)
	}
}

// ReadSubimage returns the pixel data of the rectangle r from the
// image identified by imageID src.
//
// It sends /dev/draw/n/data the message:
//	r id[4] r[4*4]
//
// and then reads the data from /dev/draw/n/data.
func (d *DrawCtrler) ReadSubimage(src uint32, r image.Rectangle) []uint8 {
	rSize := r.Size()
	msg := make([]byte, 20)
	pixels := make([]byte, (rSize.X * rSize.Y * 4))

	if (rSize.X * rSize.Y * 4) < d.iounitSize {
		binary.LittleEndian.PutUint32(msg[0:], src)
		binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
		binary.LittleEndian.PutUint32(msg[8:], uint32(r.Min.Y))
		binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
		binary.LittleEndian.PutUint32(msg[16:], uint32(r.Max.Y))

		d.sendMessage('r', msg)

		_, err := d.data.Read(pixels)
		if err != nil {
			panic(err)
		}
		return pixels
	}
	// This has the same limitation of the 'y' command.
	// Trying to read more than iounit size will return 0 bytes
	// and an Eshortread error.
	// So, again, split it up into multiple reads and reconstruct
	// it.
	// There's no compressed variant for 'r'.
	binary.LittleEndian.PutUint32(msg[0:], src)
	binary.LittleEndian.PutUint32(msg[4:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[12:], uint32(r.Max.X))
	lineSize := d.iounitSize / 4 / rSize.X

	for i := r.Min.Y; i < r.Max.Y; i += lineSize {
		endline := i + lineSize
		if endline > r.Max.Y {
			endline = r.Max.Y
		}
		binary.LittleEndian.PutUint32(msg[8:], uint32(i))
		binary.LittleEndian.PutUint32(msg[16:], uint32(endline))
		pixelsOffset := (i - r.Min.Y) * rSize.X * 4
		d.sendMessage('r', msg)
		_, err := d.data.Read(pixels[pixelsOffset:])
		if err != nil {
			panic(err)
		}
	}
	return pixels
}

// Resizes dstid to be bound by r and changes the repl bit to
// repl. This is mostly used when a window is resized.
func (d *DrawCtrler) Reclip(dstid uint32, repl bool, r image.Rectangle) {
	msg := make([]byte, 21)

	binary.LittleEndian.PutUint32(msg[0:], dstid)
	if repl {
		msg[4] = 1
	}
	binary.LittleEndian.PutUint32(msg[5:], uint32(r.Min.X))
	binary.LittleEndian.PutUint32(msg[9:], uint32(r.Min.Y))
	binary.LittleEndian.PutUint32(msg[13:], uint32(r.Max.X))
	binary.LittleEndian.PutUint32(msg[17:], uint32(r.Max.Y))
	d.sendMessage('c', msg)

}

// parseCtlString parses the output of the format returned by /dev/draw/new.
// It can also be used to parse a /dev/draw/n/ctl output, but isn't currently.
func parseCtlString(drawString string) *DrawCtlMsg {
	pieces := strings.Fields(drawString)
	if len(pieces) != 12 {
		fmt.Fprintf(os.Stderr, "Invalid /dev/draw ctl string: %s\n", drawString)
		return nil
	}
	return &DrawCtlMsg{
		N:              strToInt(pieces[0]),
		DisplayImageId: strToInt(pieces[1]),
		ChannelFormat:  pieces[2],
		// the man page says there are 12 strings returned by /dev/draw/new,
		// and in fact there are, but I only count 11 described in the man page
		// pieces[3] seems to be the location of the mystery value.
		// It seems to be "0" when I just do a cat /dev/draw/new
		MysteryValue: pieces[3],
		DisplaySize: image.Rectangle{
			Min: image.Point{strToInt(pieces[4]), strToInt(pieces[5])},
			Max: image.Point{strToInt(pieces[6]), strToInt(pieces[7])},
		},
		Clipping: image.Rectangle{
			Min: image.Point{strToInt(pieces[8]), strToInt(pieces[9])},
			Max: image.Point{strToInt(pieces[10]), strToInt(pieces[11])},
		},
	}
}

// helper function for parseCtlstring that returns a single value instead of a multi-value
// so that it can be used inline..
func strToInt(s string) int {
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return i
}
