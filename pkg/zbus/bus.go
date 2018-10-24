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

// zbusdoc
package zbus

import (
	"time"
)

const (
	MaxI2C = 9
	MaxPin = 999

	Version = "0.1.0"
)

const (
	PacketEvent     eventType = iota // indicates an incoming packet
	ErrorEvent      eventType = iota // indicates asynchronous bus error TODO?
	ConnectEvent    eventType = iota
	DisconnectEvent eventType = iota
)

const (
	AckError errorType = iota
	CrcError errorType = iota
)

type eventType byte
type errorType byte

type ZBus struct {
	Events <-chan Event
	Err    error

	bus    *i2c
	alert  *alert
	ticker *time.Ticker
	work   chan func() error
	done   chan struct{}
}

type Event struct {
	Type eventType
	Err  errorType
	Addr uint8
	Pkt  Packet
}

type Packet struct {
	Addr uint8
	Data []uint8
}

func NewZBus(dev, pin int) (*ZBus, error) {
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

	b := &ZBus{
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

func (b *ZBus) Close() {
	close(b.done)
}

func (b *ZBus) Reset() {
	b.work <- b.bus.reset
}

func (b *ZBus) Send(pkt Packet) {
	b.work <- func() error { return b.bus.send(pkt) }
}

func (b *ZBus) processWork(events chan Event) {
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
