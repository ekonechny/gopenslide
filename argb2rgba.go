package gopenslide

import (
	"bytes"

	"github.com/reiver/go-endian"
)

var (
	cb int
	cr int
	cg int
	ca int
)

func argb2rgba(data []byte) {
	buff := bytes.NewBuffer(data)
	for {
		nxt := buff.Next(4)
		if len(nxt) < 4 {
			break
		}
		a := nxt[ca]
		r := nxt[cr]
		g := nxt[cg]
		b := nxt[cb]

		nxt[0] = r & 255
		nxt[1] = g & 255
		nxt[2] = b & 255
		nxt[3] = a & 255
	}
}

func init() {
	endianness := endian.NativeEndianness()
	switch endianness {
	case endian.Big():
		ca = 0
		cr = 1
		cg = 2
		cb = 3
	default:
		cb = 0
		cg = 1
		cr = 2
		ca = 3
	}
}
