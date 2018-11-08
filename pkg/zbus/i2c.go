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

type i2c struct {
	fd  int
	arp *arp
}

// represents struct i2c_msg from <linux/i2c-dev.h>
type i2cMsg struct {
	addr  uint16
	flags uint16
	len   uint16
	_     uint16
	buf   uintptr
}

// represents struct i2c_rdwr_ioctl_data from <linux/i2c-dev.h>
type i2cRdwrIoctlData struct {
	msgs  uintptr
	nmsgs uint32
}

func newI2C(dev int, arp *arp) (*i2c, error) {
	path := fmt.Sprintf("/dev/i2c-%v", dev)

	fd, err := syscall.Open(path, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %v", path, err)
	}

	return &i2c{fd, arp}, nil
}

func (b *i2c) close() {
	syscall.Close(b.fd)
}

func (b *i2c) reset() error {
	_, err := b.transfer(CallAddr, false, []byte{0})
	return err
}

func (b *i2c) send(events chan<- Event, pkt Packet) error {
	s := b.arp.slave(pkt.Addr)
	if s == nil {
		events <- Event{Type: ErrorEvent, Err: AckError, Addr: pkt.Addr}
	}

	ok, err := b.transfer(pkt.Addr, false, pkt.Data)
	if err != nil {
		return err
	}

	if ok {
		s.touch()
	} else {
		events <- Event{Type: ErrorEvent, Err: AckError, Addr: pkt.Addr}
	}

	return nil
}

func (b *i2c) poll(events chan<- Event, a *arp) error {
	// perform poll transaction first
	buf := make([]byte, 2)
	if ok, err := b.transfer(PollAddr, true, buf); err != nil {
		return err
	} else if !ok {
		// no pending transfers
		return nil
	}

	// check received address and length
	addr := buf[0]
	n := uint8(buf[1])

	s := a.slave(addr)
	if s == nil || n < 1 || n > MaxPacketSize {
		events <- Event{Type: ErrorEvent, Err: BusError}
		return nil
	}

	// read data from the slave
	data := make([]byte, n)
	ok, err := b.transfer(addr, true, data)
	if err != nil {
		return err
	}

	if ok {
		s.touch()
		events <- Event{Type: PacketEvent, Pkt: &Packet{addr, data}}
	} else {
		events <- Event{Type: ErrorEvent, Err: AckError, Addr: addr}
	}

	return nil
}

func (b *i2c) discover(events chan<- Event) error {
	// ping silent slaves
	if err := b.ping(events); err != nil {
		return err
	}

	// discover non-configured slaves
	disc := make([]byte, 9) // UDID + Address
	for {
		if ok, err := b.transfer(ConfAddr, true, disc); err != nil {
			return err
		} else if !ok {
			// no one answered
			return nil
		}

		// someone answered
		dev := &Device{}
		copy(dev.Id[:], disc)

		s, err := b.arp.register(dev)
		if err != nil {
			// failed to register new slave
			events <- Event{Type: ErrorEvent, Err: RegError}
			return nil
		}

		events <- Event{Type: ConnectEvent, Addr: s.addr, Dev: dev}
	}
}

func (b *i2c) ping(events chan<- Event) error {
	for _, s := range b.arp.slaves {
		if s == nil || s.active() {
			continue
		}

		// perform "ping" transaction
		ok, err := b.transfer(s.addr, false, make([]byte, 0))
		if err != nil {
			return err
		}

		if ok {
			s.touch()
			continue
		}

		// slave did not answered
		// TODO(mbenda): error counter?
		b.arp.unregister(s)
		events <- Event{Type: DisconnectEvent, Addr: s.addr}
	}

	return nil
}

func (b *i2c) transfer(addr Address, read bool, data []byte) (bool, error) {
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
		// TODO(mbenda): determine which errors are fatal... or count number of successive errors
		return false, nil
	}

	return true, nil
}
