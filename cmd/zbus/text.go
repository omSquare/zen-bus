// Copyright (c) 2018 omSquare s.r.o.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"github.com/omSquare/zen-bus/pkg/zbus"
	"io"
	"strconv"
)

const pktWidth = 32 // number of bytes per packet data line

var hex = [...]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F'}

// TextProtocol is a command based bus protocol that reads commands from an io.Reader and writes command
// to an io.Writer.
type TextProtocol struct {
	r   io.Reader
	w   io.Writer
	buf []byte
	pos int
	n   int
}

// NewTextProtocol creates a new TextProtocol for the specified reader and writer.
func NewTextProtocol(r io.Reader, w io.Writer) *TextProtocol {
	return &TextProtocol{
		r:   r,
		w:   w,
		buf: make([]byte, 1024),
	}
}

// TODO write funcs errors?

// WriteVersion outputs the "welcome" command.
func (p *TextProtocol) WriteVersion(ver string) {
	fmt.Fprintf(p.w, "ZBUS %v\n", ver)
}

// WriteError outputs the "ERR" command.
func (p *TextProtocol) WriteError(addr uint8) {
	fmt.Fprintf(p.w, "ERR %02X\n", addr)
}

// WriteReset outputs the "RST" command.
func (p *TextProtocol) WriteReset() {
	fmt.Fprint(p.w, "RST\n")
}

// WritePacket outputs the "PKT" command.
func (p *TextProtocol) WritePacket(pkt zbus.Packet) {
	fmt.Fprintf(p.w, "PKT %02X %02X\n", pkt.Addr, len(pkt.Data))
	var line [pktWidth*2 + 1]byte

	for i := 0; i < len(pkt.Data); {
		l := i + pktWidth
		if l > len(pkt.Data) {
			l = len(pkt.Data)
		}

		k := 0
		for j := i; j < l; j++ {
			b := pkt.Data[j]
			line[k] = hex[b/16]
			line[k+1] = hex[b%16]
			k += 2
		}

		line[k] = '\n'
		p.w.Write(line[:k+1])
		i = l
	}
}

// WriteConnect outputs the "CONN" command.
func (p *TextProtocol) WriteConnect(addr uint8) {
	fmt.Fprintf(p.w, "CONN %02X\n", addr)
}

// WriteDisconnect outputs the "DISC" command.
func (p *TextProtocol) WriteDisconnect(addr uint8) {
	fmt.Fprintf(p.w, "DISC %02X\n", addr)
}

// Read reads the next command form the protocol input.
func (p *TextProtocol) Read() (Command, error) {
	// read command token first
	cmd, err := p.nextToken()
	if err != nil {
		return Command{}, err
	}

	switch cmd {
	case "RST":
		return Command{Type: CmdReset}, nil

	case "PKT":
		return p.readPacket()

	default:
		return Command{}, ErrProto
	}
}

func (p *TextProtocol) readPacket() (Command, error) {
	// read address, length
	addr, err := p.nextByte()
	if err != nil {
		return Command{}, ErrProto
	}

	n, err := p.nextByte()
	if err != nil {
		return Command{}, ErrProto
	}

	// validate length
	if n < 1 || n > zbus.MaxPacketSize {
		return Command{}, ErrProto
	}

	// read data
	pkt := zbus.Packet{addr, make([]uint8, n)}
	for i := 0; i < len(pkt.Data); {
		tok, err := p.nextToken()
		if err != nil || len(tok)%2 == 1 || i+len(tok)/2 > len(pkt.Data) {
			return Command{}, ErrProto
		}

		for j := 0; j < len(tok); j += 2 {
			h := hexDigit(tok[j])
			l := hexDigit(tok[j+1])

			if h < 0 || l < 0 {
				return Command{}, ErrProto
			}

			pkt.Data[i] = uint8(16*h + l)
			i++
		}
	}

	return Command{Type: CmdPacket, Pkt: pkt}, nil
}

func (p *TextProtocol) nextByte() (uint8, error) {
	tok, err := p.nextToken()
	if err != nil {
		return 0, ErrProto
	}

	n, err := strconv.ParseUint(tok, 16, 8)
	if err != nil {
		return 0, ErrProto
	}

	return uint8(n), nil
}

func (p *TextProtocol) nextToken() (string, error) {
	var tok [32]byte
	var i int

	for {
		// skip leading whitespace
		for p.pos < p.n && isSpace(p.buf[p.pos]) {
			p.pos++
		}

		// read token characters
		for p.pos < p.n && i < len(tok) && !isSpace(p.buf[p.pos]) {
			tok[i] = p.buf[p.pos]
			i++
			p.pos++
		}

		// re-read buffer if needed
		if p.pos == p.n {
			var err error

			p.pos = 0
			p.n, err = p.r.Read(p.buf)

			if p.n == 0 {
				return "", err
			}
		} else {
			break
		}
	}

	return string(tok[:i]), nil
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\v' || ch == '\f'
}

func hexDigit(ch byte) int {
	if ch >= '0' && ch <= '9' {
		return int(ch - '0')
	}

	if ch >= 'A' && ch <= 'F' {
		return int(ch - 'A')
	}

	return -1
}
