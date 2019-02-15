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
	"time"
)

const (
	// Version contains the zen-bus version.
	Version = "0.1.0"

	// MaxI2C is the maximum number of an I2C device.
	MaxI2C = 9

	// MaxPin is the maximum number of a GPIO alert pin.
	MaxPin = 999

	// MaxPacketSize defines the maximum payload size of a packet.
	MaxPacketSize = 128

	// EventCapacity defines the size of the Events channel.
	EventCapacity = 8

	// MaxSlaves determines the maximum number of connected slave devices.
	MaxSlaves = 32
)

const (
	// CallAddr is the general call address. All slave devices listen to this broadcast address.
	CallAddr Address = 0x00 // general call address, used for bus reset

	// ConfAddr is a broadcast address used to configure slave devices that connect to the bus.
	ConfAddr Address = 0x76

	// PollAddr is a broadcast address that all registered slaves listen on. Any slave device that has data to send will
	// answer this address.
	PollAddr Address = 0x77
)

const (
	// PacketEvent indicates an incoming packet.
	PacketEvent eventType = iota

	// ErrorEvent indicates asynchronous bus error.
	ErrorEvent eventType = iota

	// ConnectEvent indicates that a new slave device connected the bus.
	ConnectEvent eventType = iota

	// DisconnectEvent indicates that a device disconnected from the bus.
	DisconnectEvent eventType = iota
)

const (
	// BusError indicates that a generic bus error occurred.
	BusError errorType = iota

	// AckError indicates that a transaction was not acked properly. The Addr field determines the slave.
	AckError errorType = iota

	// CrcError indicates that a CRC packet error occurred. The Addr field determines the slave.
	CrcError errorType = iota

	// RegError indicates that a slave device could not register properly.
	RegError errorType = iota
)

// Bus holds a channel that delivers asynchronous bus events.
type Bus struct {
	Events <-chan Event
	Err    error

	ev     chan Event
	driver Driver
	alert  *alert
	arp    *arp
	ticker *time.Ticker
	work   chan func() error
	done   chan struct{}
}

// Event represents an asynchronous bus event.
type Event struct {
	Type eventType
	Err  errorType
	Addr Address
	Pkt  *Packet
	Dev  *Device
}

// Address of a slave device.
type Address = uint8

// Packet consists of a destination address and data payload.
type Packet struct {
	Addr Address
	Data []byte
}

// Udid stands for Unique Device Identifier.
type Udid = [8]byte

// Device is a slave device descriptor.
type Device struct {
	Id Udid
	// TODO(mbenda): flags, serial number etc.
}

type eventType byte
type errorType byte

// Driver handles low-level bus communication.
type Driver interface {
	// Close closes the bus.
	Close()

	// Reset resets the bus.
	Reset() error

	// Discover discovers new slave devices on the bus. Discovered clients are sent as events to the provided channel.
	Discover(events chan<- Event) error

	// Poll polls for data packets from slave devices. Received packets are sent to the provided channel as packet
	// events.
	Poll(events chan<- Event) error

	// Sends sends a packet to a slave device. Eventual errors are sent to the provided channel as error events.
	Send(events chan<- Event, pkt Packet) error
}

// TODO(mbenda): logging

// New creates and returns a new Bus for the specified I2C device number and alert GPIO pin.
func New(dev, pin int) (*Bus, error) {
	// check parameters
	if dev < 0 || dev > MaxI2C {
		return nil, errors.New("invalid I2C device index")
	}

	if pin < 0 || pin > MaxPin {
		return nil, errors.New("invalid GPIO pin index")
	}

	// prepare ARP
	arp := &arp{}

	// init GPIO alert and I2C bus
	bus, err := newI2C(dev, arp)
	if err != nil {
		return nil, err
	}

	alert, err := newAlert(pin)
	if err != nil {
		return nil, err
	}

	events := make(chan Event, EventCapacity)

	b := &Bus{
		Events: events,
		ev:     events,
		alert:  alert,
		arp:    arp,
		driver: bus,
		ticker: time.NewTicker(time.Second),
		work:   make(chan func() error),
		done:   make(chan struct{}),
	}

	go alert.watch()
	go b.processWork()

	return b, nil
}

// Close closes the bus.
func (b *Bus) Close() {
	close(b.done)
}

// Reset resets the state of the bus.
func (b *Bus) Reset() {
	b.work <- b.driver.Reset
}

// Send sends a packet on the bus.
func (b *Bus) Send(pkt Packet) {
	b.work <- func() error { return b.driver.Send(b.ev, pkt) }
}

func (b *Bus) processWork() {
	defer func() {
		b.ticker.Stop()
		b.alert.close()
		b.driver.Close()
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
			if err := b.driver.Discover(b.ev); err != nil {
				b.Err = err
				return
			}

		case fn := <-b.work:
			// do some work
			if err := fn(); err != nil {
				// terminate with the error
				b.Err = err
				return
			}

		case s, ok := <-b.alert.state:
			if !ok {
				b.Err = b.alert.err
				return
			}
			alert = s == 0
		}

		// process alert, not more than MaxSlaves in a row
		limit := MaxSlaves

		for alert {
			if err := b.driver.Poll(b.ev); err != nil {
				b.Err = err
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
					b.Err = b.alert.err
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
