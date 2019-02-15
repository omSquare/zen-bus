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
	// SysError represents an unrecoverable system error that prevents the bus from functioning properly. The client
	// must close the bus after receiving this error event. TODO proper error passing.
	SysError errorType = iota

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
type Bus interface {
	// Close closes the bus asynchronously.
	Close()

	// Reset resets the state of the bus asynchronously.
	Reset()

	// Send sends a packet on the bus asynchronously.
	Send(pkt Packet)

	// Events provides access to bus events.
	Events() <-chan Event
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

// TODO(mbenda): logging
