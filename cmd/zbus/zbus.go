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

// Command zbus provides zen-bus interface via stdin/stdout.
package main

import (
	"errors"
	"fmt"
	"github.com/omSquare/zen-bus/pkg/zbus"
	"io"
	"os"
	"strconv"
)

const (
	exitUsage = 64
	exitIOErr = 74
)

type input struct {
	cmd Command
	err error
}

func main() {
	dev, pin := parseCmdLine()

	b, err := zbus.NewBus(dev, pin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitIOErr)
	}

	proto := NewProtocol(os.Stdin, os.Stdout)
	input := readCommands(proto)

	proto.WriteVersion(zbus.Version)

	for err = error(nil); err == nil; {
		select {
		case in, ok := <-input:
			if in.err != nil || !ok {
				err = in.err
			} else {
				err = processCommand(b, in.cmd)
			}

		case ev, ok := <-b.Events:
			if !ok {
				err = b.Err
			} else {
				err = processEvent(proto, ev)
			}
		}
	}

	b.Close()

	if err != nil && err != io.EOF {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitIOErr)
	}
}

func parseCmdLine() (dev, pin int) {
	if len(os.Args) != 3 {
		usage()
	}

	dev, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: invalid I2C device number")
		usage()
	}

	pin, err = strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: invalid GPIO pin number")
		usage()
	}

	return
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %v <i2c_num> <gpio_num>\n", os.Args[0])
	os.Exit(exitUsage)
}

func readCommands(proto *Protocol) chan input {
	ch := make(chan input)
	go func() {
		for {
			cmd, err := proto.Read()
			if err != nil {
				ch <- input{Command{}, err}
				close(ch)
				return
			}

			ch <- input{cmd, nil}
		}
	}()

	return ch
}

func processCommand(b *zbus.Bus, cmd Command) error {
	switch cmd.Type {
	case CmdReset:
		b.Reset()

	case CmdPacket:
		b.Send(cmd.Pkt)
	}
	return nil
}

func processEvent(p *Protocol, ev zbus.Event) error {
	switch ev.Type {
	case zbus.PacketEvent:
		p.WritePacket(ev.Pkt)

	case zbus.ErrorEvent:
		p.WriteError(ev.Addr)

	case zbus.ConnectEvent:
		p.WriteConnect(ev.Addr)

	case zbus.DisconnectEvent:
		p.WriteDisconnect(ev.Addr)

	default:
		return errors.New("unsupported bus event")
	}

	return nil
}
