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

package zbus

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	addrCall uint8 = 0x00
	addrConf uint8 = 0x76
	addrPoll uint8 = 0x77
)

type i2c struct {
	fd int
}

type i2cMsg struct {
	addr  uint16
	flags uint16
	len   uint16
	_     uint16
	buf   uintptr
}

type i2cRdwrIoctlData struct {
	msgs  uintptr
	nmsgs uint32
}

func newI2C(dev int) (*i2c, error) {
	path := fmt.Sprintf("/dev/i2c-%v", dev)

	fd, err := syscall.Open(path, syscall.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	return &i2c{fd}, nil
}

func (b *i2c) close() {
	syscall.Close(b.fd)
}

func (b *i2c) reset() error {
	// TODO error handling
	return b.transfer(addrCall, false, []uint8{0})
}

func (b *i2c) send(pkt Packet) error {
	// TODO error handling
	return b.transfer(pkt.Addr, false, pkt.Data)
}

func (b *i2c) poll() (Packet, error) {
	// perform poll transaction first
	buf := make([]uint8, 2)
	if err := b.transfer(addrPoll, true, buf); err != nil {
		// TODO error handling
		return Packet{}, err
	}

	// check received address and length
	addr := buf[0]
	n := buf[1]
	// TODO: validate

	// read data from the slave
	data := make([]uint8, n)
	if err := b.transfer(addr, true, data); err != nil {
		// TODO error handling
		return Packet{}, err
	}

	return Packet{addr, data}, nil
}

func (b *i2c) discover() error {
	// TODO
	return nil
}

func (b *i2c) transfer(addr uint8, read bool, data []uint8) error {
	const (
		I2C_M_RD = 0x0001
		I2C_RDWR = 0x0707
	)

	// prepare message
	msg := i2cMsg{
		addr: uint16(addr),
		len:  uint16(len(data)),
		buf:  uintptr(unsafe.Pointer(&data[0])),
	}

	if read {
		msg.flags = I2C_M_RD
	}

	// prepare RDWR ioctl data
	rdwr := i2cRdwrIoctlData{uintptr(unsafe.Pointer(&msg)), 1}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(b.fd), uintptr(I2C_RDWR), uintptr(unsafe.Pointer(&rdwr)))
	if errno != 0 {
		return errno
	}

	return nil
}
