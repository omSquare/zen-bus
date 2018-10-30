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
	// PacketEvent indicates an incoming packet.
	PacketEvent eventType = iota

	// ErrorEvent indicates asynchronous bus error.
	ErrorEvent eventType = iota // TODO split between bus error and packer error?

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
)

// Bus holds a channel that delivers asynchronous bus events.
type Bus struct {
	Events <-chan Event
	Err    error

	ev     chan Event
	arp    *arp
	bus    *i2c
	alert  *alert
	ticker *time.Ticker
	work   chan func() error
	done   chan struct{}
}

// Event represents an asynchronous bus event.
type Event struct {
	Type eventType
	Err  errorType
	Addr Address
	Pkt  Packet // TODO(mbenda): pointer?
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

// New creates and returns a new Bus for the specified I2C device number and alert GPIO pin.
func New(dev, pin int) (*Bus, error) {
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
		bus:    bus,
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
	b.work <- b.bus.reset
}

// Send sends a packet on the bus.
func (b *Bus) Send(pkt Packet) {
	b.work <- func() error { return b.bus.send(b.ev, pkt) }
}

func (b *Bus) processWork() {
	defer func() {
		b.ticker.Stop()
		b.alert.close()
		b.bus.close()
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
			if err := b.bus.discover(b.ev); err != nil {
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

		// process alert
		for alert {
			if err := b.bus.poll(b.ev, b.arp); err != nil {
				b.Err = err
				return
			}

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
