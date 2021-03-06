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
	"errors"
	"github.com/omSquare/zen-bus/pkg/zbus"
)

const (
	// CmdReset represents the "RST" command.
	CmdReset = iota

	// CmdPacket represents the "PKT" command.
	CmdPacket = iota
)

// Command received by the protocol.
type Command struct {
	Type int
	Pkt  zbus.Packet
}

// Protocol defines a contract for reading and writing commands.
type Protocol interface {
	Read() (Command, error)
	WriteVersion(ver string)
	WriteError(addr uint8)
	WriteReset()
	WritePacket(pkt zbus.Packet)
	WriteConnect(addr uint8)
	WriteDisconnect(addr uint8)
}

// ErrProto indicates a protocol violation error.
var ErrProto = errors.New("protocol violation")
