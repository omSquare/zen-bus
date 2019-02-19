// Copyright (c) 2019 omSquare s.r.o.
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
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

const (
	magic   uint16 = 0x7082
	version uint16 = 0x0000

	cmdPacket uint8 = 0x00
	cmdConf   uint8 = 0x01
	cmdQuit   uint8 = 0xFF
)

// SimBus is a simulated Zbus implementation that creates a TCP server
type SimBus struct {
	ev   chan Event
	work chan func() error
	conn chan client
	disc chan client
	done chan struct{}
	term chan struct{}

	addr    string
	server  *net.TCPListener
	clients map[Address]client

	arp arp
}

type client struct {
	conn *net.TCPConn
	dev  *Device
	addr Address
}

func NewSimBus(addr string) (*SimBus, error) {
	b := &SimBus{
		ev:   make(chan Event, EventCapacity),
		work: make(chan func() error),
		conn: make(chan client),
		disc: make(chan client),
		done: make(chan struct{}),
		term: make(chan struct{}),

		addr:    addr,
		clients: make(map[Address]client),
	}

	go b.processWork()
	b.Reset()

	return b, nil
}

// Close closes the simulated bus.
func (b *SimBus) Close() {
	defer func() {
		// handle the case when the done/term channels are already closed
		_ = recover()
	}()

	log.Println("closing bus")
	close(b.done)
	<-b.term
}

// Reset resets the simulated bus by closing and re-opening the server
func (b *SimBus) Reset() {
	b.work <- func() error {
		log.Println("resetting bus")

		b.closeAll()

		// reset ARP and re-open server
		b.clients = make(map[Address]client)
		b.arp = arp{}

		ln, err := net.Listen("tcp", b.addr)
		if err != nil {
			return err
		}

		b.ev <- Event{Type: ResetEvent}
		b.server = ln.(*net.TCPListener)

		go b.processServer(b.server)

		return nil
	}
}

func (b *SimBus) Send(pkt Packet) {
	b.work <- func() error {
		log.Printf("sending packet %v\n", pkt)

		// find client connection
		// TODO(mbenda): check ARP?
		cl, ok := b.clients[pkt.Addr]
		if !ok {
			// client not found
			b.ev <- Event{Type: ErrorEvent, Err: AckError, Addr: pkt.Addr}
			return nil
		}

		// and send the packet
		data := []byte{cmdPacket, pkt.Len()}
		data = append(data, pkt.Data...)

		_, err := cl.conn.Write(data)
		if err != nil {
			b.ev <- Event{Type: ErrorEvent, Err: AckError, Addr: pkt.Addr}
			return nil
		}

		return nil
	}
}

func (b *SimBus) Events() <-chan Event {
	return b.ev
}

func (b *SimBus) closeAll() {
	// close the listener and all client connections
	if b.server != nil {
		_ = b.server.Close()
	}

	for _, cl := range b.clients {
		closeClient(cl)
	}
}

func (b *SimBus) processWork() {
	defer func() {
		log.Println("terminating")
		b.closeAll()
		close(b.term)
	}()

	for {
		select {
		case <-b.done:
			// we are done here
			return

		case fn := <-b.work:
			if err := fn(); err != nil {
				log.Println(err)
				b.ev <- Event{Type: ErrorEvent, Err: SysError} // TODO(mbenda): error reporting
				return
			}

		case c := <-b.conn:
			// new incoming connection
			log.Printf("registering new client %v\n", *c.dev)

			slave, err := b.arp.register(c.dev)
			if err != nil {
				// reject the client
				log.Println("rejecting client:", err)

				_ = c.conn.Close()
				b.ev <- Event{Type: ErrorEvent, Err: RegError}
				continue
			}

			prev, ok := b.clients[slave.addr]
			if ok {
				// drop previous client
				log.Printf("closing previous client connection from %v\n", c.conn.RemoteAddr())

				closeClient(prev)
			}

			c.addr = slave.addr
			b.clients[slave.addr] = c

			b.ev <- Event{Type: ConnectEvent, Addr: c.addr}

			go b.processSlave(c)

		case c := <-b.disc:
			// client disconnected
			prev, ok := b.clients[c.addr]
			if ok && prev == c {
				// unregister it
				delete(b.clients, c.addr)
				b.arp.unregister(b.arp.slave(c.addr))

				b.ev <- Event{Type: DisconnectEvent, Addr: c.addr}
			}
		}
	}
}

func (b *SimBus) processServer(ln *net.TCPListener) {
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			break
		}

		go b.processHandshake(conn)
	}
}

func (b *SimBus) processHandshake(conn *net.TCPConn) {
	log.Printf("new connection from %v", conn.RemoteAddr())

	// write server handshake
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data, magic)
	binary.BigEndian.PutUint16(data[2:], version)
	_, err := conn.Write(data)
	if err != nil {
		log.Println("client handshake error:", err)
		return
	}

	// read client handshake
	udid, err := parseHandshake(conn)
	if err != nil {
		log.Println("client handshake error:", err)
		_ = conn.Close()
		return
	}

	// let the main loop register the client
	b.conn <- client{conn: conn, dev: &Device{Id: udid}}
}

func parseHandshake(conn *net.TCPConn) (Udid, error) {
	buf := make([]byte, 4)
	n, err := conn.Read(buf)
	if err != nil {
		return Udid{}, err
	}

	// check header
	if n != 4 || binary.BigEndian.Uint16(buf) != magic {
		return Udid{}, errors.New("no magic")
	}

	ver := binary.BigEndian.Uint16(buf[2:])
	if (ver & 0xFF00) != (version & 0xFF00) {
		return Udid{}, errors.New(fmt.Sprintf("incompatible versions (client: %04x, server: %04x)", ver, version))
	}

	// read UDID
	var udid Udid
	n, err = conn.Read(udid[:])
	if err != nil {
		return Udid{}, err
	}

	if n != len(udid) {
		return Udid{}, errors.New("invalid UDID")
	}

	return udid, nil
}

func (b *SimBus) processSlave(c client) {
	defer func() {
		b.disc <- c
		_ = c.conn.Close()
	}()

	log.Printf("processing connection from %v", c.conn.RemoteAddr())

	// let the client know its address
	_, err := c.conn.Write([]byte{cmdConf, c.addr})
	if err != nil {
		log.Println("client I/O error:", err)
		b.ev <- Event{Type: ErrorEvent, Err: BusError}
		return
	}

	// process packets
	for {
		// read packet header
		var header [2]byte
		n, err := c.conn.Read(header[:])
		if err == io.EOF {
			// client disconnected
			return
		}

		if err != nil {
			log.Println("client I/O error:", err)
			b.ev <- Event{Type: ErrorEvent, Err: BusError}
			return
		}

		if n != 2 || header[0] != cmdPacket {
			log.Println("client I/O error: protocol violation")
			b.ev <- Event{Type: ErrorEvent, Err: BusError}
			return
		}

		// read packet data
		pkt := Packet{Addr: c.addr, Data: make([]byte, int(header[1]))}
		for i := 0; i < len(pkt.Data); {
			n, err = c.conn.Read(pkt.Data[i:])
			if err != nil {
				log.Println("client I/O error:", err)
				b.ev <- Event{Type: ErrorEvent, Err: BusError}
				return
			}

			i += n
		}

		b.ev <- Event{Type: PacketEvent, Pkt: &pkt}
	}
}

func closeClient(c client) {
	// deliberately ignore errors
	_, _ = c.conn.Write([]byte{cmdQuit})
	_ = c.conn.CloseRead()
}
