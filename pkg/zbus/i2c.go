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
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// I2CBus implements the Bus interface using I2C and GPIO.
type I2CBus struct {
	ev chan Event

	ticker *time.Ticker
	work   chan func() error
	done   chan struct{}
	arp    *arp

	i2c   int
	alert *gpio
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

// NewI2CBus creates a new I2C and GPIO based Bus instance.
func NewI2CBus(dev int, pin int) (*I2CBus, error) {
	// check parameters
	if dev < 0 || dev > MaxI2C {
		return nil, errors.New("invalid I2C device index")
	}

	if pin < 0 || pin > MaxPin {
		return nil, errors.New("invalid GPIO pin index")
	}

	// open I2C device
	i2cPath := fmt.Sprintf("/dev/i2c-%v", dev)
	i2c, err := syscall.Open(i2cPath, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %v", i2cPath, err)
	}

	// open GPIO alert pin
	alert, err := newGpio(pin)
	if err != nil {
		return nil, err
	}

	b := &I2CBus{
		ev: make(chan Event, EventCapacity),

		ticker: time.NewTicker(time.Second),
		work:   make(chan func() error),
		done:   make(chan struct{}),

		arp:   &arp{},
		i2c:   i2c,
		alert: alert,
	}

	return b, nil
}

// Close closes the I2C bus.
func (b *I2CBus) Close() {
	close(b.done)
}

// Reset resets the I2C bus by sending the reset command.
func (b *I2CBus) Reset() {
	b.work <- func() error {
		_, err := b.transfer(CallAddr, false, []byte{0})
		return err
	}
}

// Send sends a packet to the I2C bus.
func (b *I2CBus) Send(pkt Packet) {
	b.work <- func() error {
		s := b.arp.slave(pkt.Addr)
		if s == nil {
			b.ev <- Event{Type: ErrorEvent, Err: AckError, Addr: pkt.Addr}
		}

		ok, err := b.transfer(pkt.Addr, false, pkt.Data)
		if err != nil {
			return err
		}

		if ok {
			s.touch()
		} else {
			b.ev <- Event{Type: ErrorEvent, Err: AckError, Addr: pkt.Addr}
		}

		return nil
	}
}

// Events provides access to the channel of bus events.
func (b *I2CBus) Events() <-chan Event {
	return b.ev
}

func (b *I2CBus) processWork() {
	defer func() {
		b.ticker.Stop()
		b.alert.close()
		_ = syscall.Close(b.i2c)
		close(b.ev)
	}()

	var alert bool

	for {
		// wait for next event
		select {
		case <-b.done:
			// we are done here
			return

		case <-b.ticker.C:
			if err := b.discover(); err != nil {
				b.ev <- Event{Type: ErrorEvent, Err: SysError} // TODO err
				return
			}

		case fn := <-b.work:
			// do some work
			if err := fn(); err != nil {
				// terminate with the error
				b.ev <- Event{Type: ErrorEvent, Err: SysError} // TODO err
				return
			}

		case s, ok := <-b.alert.state:
			if !ok {
				b.ev <- Event{Type: ErrorEvent, Err: SysError} // TODO b.alert.err
				return
			}
			alert = s == 0
		}

		// process alert, not more than MaxSlaves in a row
		limit := MaxSlaves

		for alert {
			if err := b.poll(); err != nil {
				b.ev <- Event{Type: ErrorEvent, Err: SysError} // TODO err
				return
			}

			if limit == 0 {
				// stop processing alerts TODO bus error instead?
				break
			}

			limit--

			select {
			case s, ok := <-b.alert.state:
				if !ok {
					b.ev <- Event{Type: ErrorEvent, Err: SysError} // TODO b.alert.err
					return
				}
				alert = s == 0

			default:
				// poll for another packet
				break
			}
		}
	}
}

func (b *I2CBus) poll() error {
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

	s := b.arp.slave(addr)
	if s == nil || n < 1 || n > MaxPacketSize {
		b.ev <- Event{Type: ErrorEvent, Err: BusError}
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
		b.ev <- Event{Type: PacketEvent, Pkt: &Packet{addr, data}}
	} else {
		b.ev <- Event{Type: ErrorEvent, Err: AckError, Addr: addr}
	}

	return nil
}

func (b *I2CBus) discover() error {
	// ping silent slaves
	if err := b.ping(); err != nil {
		return err
	}

	// discover non-configured slaves
	// TODO(mbenda): some limit
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
			b.ev <- Event{Type: ErrorEvent, Err: RegError}
			return nil
		}

		b.ev <- Event{Type: ConnectEvent, Addr: s.addr, Dev: dev}
	}
}

func (b *I2CBus) ping() error {
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
		b.ev <- Event{Type: DisconnectEvent, Addr: s.addr}
	}

	return nil
}

func (b *I2CBus) transfer(addr Address, read bool, data []byte) (bool, error) {
	const (
		I2cMRd  = 0x0001
		I2cRdwr = 0x0707
	)

	// prepare message
	msg := i2cMsg{
		addr: uint16(addr),
		len:  uint16(len(data)),
		buf:  uintptr(unsafe.Pointer(&data[0])),
	}

	if read {
		msg.flags = I2cMRd
	}

	// prepare RDWR ioctl data
	rdwr := i2cRdwrIoctlData{uintptr(unsafe.Pointer(&msg)), 1}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(b.i2c), uintptr(I2cRdwr), uintptr(unsafe.Pointer(&rdwr)))
	if errno != 0 {
		// TODO(mbenda): determine which errors are fatal... or count number of successive errors
		return false, nil
	}

	return true, nil
}
