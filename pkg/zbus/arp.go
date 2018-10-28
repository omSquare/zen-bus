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
	minAddr Address = 0x10 // the lower bound of the slave address space (inclusive)
	maxAddr Address = 0x50 // the upper bound of the slave address space (exclusive)

	numAddr = maxAddr - minAddr

	silenceLimit time.Duration = 5 * time.Second
)

var (
	errTooManySlaves = errors.New("too many slaves")
)

// state of the address resolution protocol
type arp struct {
	// array of all slaves indexed by their address offset (index + minAddr == addr)
	slaves [numAddr]*slave

	// current number of slaves
	num int
}

type slave struct {
	addr     Address
	id       Udid
	lastSeen time.Time
}

func (a *arp) register(dev *Device) (*slave, error) {
	// first check if the device is not already registered
	if s := a.findSlave(dev.Id); s != nil {
		// unregister it
		// TODO(mbenda): notify client
		// TODO(mbenda): just replace instead?
		a.unregister(s)
	}

	// find a new slot (address)
	s, err := a.findAddr()
	if err != nil {
		return nil, err
	}

	a.num++
	s.id = dev.Id
	s.touch()

	return s, nil
}

func (a *arp) unregister(s *slave) {
	if a.slaves[s.index()] == s {
		a.slaves[s.index()] = nil
		a.num--
	}
}

// tries to find a slave with the given UDID
func (a *arp) findSlave(id Udid) *slave {
	for _, s := range a.slaves {
		if s != nil && s.id == id {
			return s
		}
	}
	return nil
}

// finds a free address and allocates a slave
func (a *arp) findAddr() (*slave, error) {
	if a.num == MaxSlaves {
		return nil, errTooManySlaves
	}

	// TODO(mbenda): slave priorities
	for i, s := range a.slaves {
		if s != nil {
			continue
		}

		// an empty slot was found
		a.slaves[i] = &slave{addr: minAddr + Address(i)}
		return a.slaves[i], nil
	}

	return nil, errTooManySlaves
}

func (s *slave) index() int {
	return int(s.addr - minAddr)
}

func (s *slave) active() bool {
	return time.Now().Sub(s.lastSeen) < silenceLimit
}

func (s *slave) touch() {
	s.lastSeen = time.Now()
}
