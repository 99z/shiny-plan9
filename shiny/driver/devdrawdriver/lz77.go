// Copyright 2016-2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package devdrawdriver

// Gets index and size of the largest prefix of pix[idx] which occurs
// before it in pix. If it doesn't find a prefix of at least size 3,
// it will claim it couldn't find any, and if it finds one of size 34,
// it will claim that's the largest that it found since that's the range
// that fits in a compressed image.
//
// It will search at most 128 bytes back (32 pixels) which should be enough
// to cover the common case of a pixel repeating itself in a fill colour, without
// adding too much CPU overhead in a degenerate case.
//
// If it doesn't find anything, it will return 0, 0 indicating that bytes should just be
// encoded directly.
func getLargestPrefix(pix []byte, idx int) (uint16, uint8) {
	// BUG(driusan): This length that it searches back should probably be a tuneable parameter
	// since the optimum value is going to be a function of bandwidth and CPU, but from trial
	// and error on a Raspberry Pi 2 over a wifi connection (probably close to the worst case
	// scenerio), looking back the full 1024 bytes is slower than not using compression, while
	// 128 provides some gains. More powerful CPU servers will still get gains from this, just
	// not as much as if they looked back farther.
	var candidateIdx uint16
	var candidateSize uint8
	for i := int(idx - 34); i >= 0 && (idx-i < 128); i-- {
		if pix[i] == pix[idx] {
			if idx+34 >= len(pix) {
				break
			}
			for j, val := range pix[idx : idx+34] {
				if i+j >= len(pix) {
					break
				}
				if val == pix[i+j] {
					if j > int(candidateSize) {
						candidateSize = uint8(j)
						candidateIdx = uint16(i)
					}
				} else {
					break
				}
				if candidateSize == 34 {
					return candidateIdx, candidateSize
				}

			}
		}
	}
	if candidateSize > 2 {
		return candidateIdx, candidateSize
	}
	return 0, 0
}

// Compresses pix using the variant of LZ77 compression described in image(6)
func compress(pix []byte) []byte {
	val := make([]byte, 0)
	for i := 0; i < len(pix); {
		if idx, size := getLargestPrefix(pix, i); size > 2 {
			// "If the high-order bit is zero, the next 5 bits encode the
			//  length of a substring copied from previous pixels. Values
			//  from 0 to 31 encode lengths from 3 to 34. The bottom
			//  two bits of the first byte and the 8 bits of the next byte
			//  encode an offset backward from the current position in the
			//  pixel data at which the copy is to be found. Values from
			//  0 to 1023 encode offsets from 1 to 1024."
			var encoding [2]byte

			// encode the length
			encoding[0] = (size - 3) << 2

			// encode the offset
			encodedOffset := uint16(i-int(idx)) - 1
			encoding[0] |= byte((encodedOffset & 0x0300) >> 8)
			encoding[1] = byte(encodedOffset & 0x00FF)
			val = append(val, encoding[:]...)

			i += int(size)
		} else {
			// "In a code whose first byte has the high-order bit set, the rest
			//  of the byte encodes the length of a byte encoded
			// directly. Values from 0 to 127 encode lengths from 1 to 128
			// bytes. Subsequent bytes are the literal pixel data."
			//
			// If there were no matches, we just add as much data
			// as we can in order to give the next bit pixel a better
			// chance of finding something to match against without wasting
			// much CPU time.
			if left := len(pix) - i; left >= 128 {
				val = append(val, 0xFF)
				val = append(val, pix[i:i+128]...)

				i += 128
			} else {
				val = append(val, (0x80 | byte(left-1)))
				val = append(val, pix[i:i+left]...)

				i += left
			}
		}

	}
	return val
}
