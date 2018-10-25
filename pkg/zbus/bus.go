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
	// MaxI2C is the maximum number of an I2C device.
	MaxI2C = 9
	// MaxPin is the maximum number of a GPIO alert pin.
	MaxPin = 999

	// Version contains the zen-bus version.
	Version = "0.1.0"
)

const (
	PacketEvent     eventType = iota // PacketEvent indicates an incoming packet.
	ErrorEvent      eventType = iota // ErrorEvent indicates asynchronous bus error. TODO split between bus error and packer error?
	ConnectEvent    eventType = iota // ConnectEvent indicates that a new slave device connected the bus.
	DisconnectEvent eventType = iota // DisconnectEvent indicates that a device disconnected from the bus.
)

const (
	AckError errorType = iota // AckError indicates that a transaction was not acked properly.
	CrcError errorType = iota // CrcError indicates that a CRC packet error occurred.
)

type eventType byte
type errorType byte

// Bus holds a channel that delivers asynchronous bus events.
type Bus struct {
	Events <-chan Event
	Err    error

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
	Addr uint8
	Pkt  Packet
}

// Packet consists of a destination address and data payload.
type Packet struct {
	Addr uint8
	Data []uint8
}

// NewBus creates and returns a new Bus for the specified I2C device number and alert GPIO pin.
func NewBus(dev, pin int) (*Bus, error) {
	// init GPIO alert and I2C bus
	bus, err := newI2C(dev)
	if err != nil {
		return nil, err
	}

	alert, err := newAlert(pin)
	if err != nil {
		return nil, err
	}

	events := make(chan Event)

	b := &Bus{
		Events: events,
		alert:  alert,
		bus:    bus,
		ticker: time.NewTicker(time.Second),
		work:   make(chan func() error),
		done:   make(chan struct{}),
	}

	go alert.watch()
	go b.processWork(events)

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
	b.work <- func() error { return b.bus.send(pkt) }
}

func (b *Bus) processWork(events chan Event) {
	defer func() {
		b.ticker.Stop()
		b.alert.close()
		b.bus.close()
		close(events)
	}()

	var alert bool

	for {
		// wait for next event
		select {
		case <-b.done:
			// we are done here
			return

		case <-b.ticker.C:
			if err := b.bus.discover(); err != nil {
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
			pkt, err := b.bus.poll()
			if err != nil {
				b.Err = err
				return
			}

			events <- Event{Type: PacketEvent, Pkt: pkt}

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
