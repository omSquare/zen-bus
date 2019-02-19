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
	"os/signal"
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
	if len(os.Args) == 1 {
		printHelp()
		os.Exit(exitUsage)
	}

	var (
		b   zbus.Bus
		err error
	)

	switch os.Args[1] {
	case "i2c":
		b, err = createI2CBus()

	case "sim":
		b, err = createSimBus()

	default:
		printErr("error: invalid bus type '%s'", os.Args[1])
		os.Exit(exitUsage)
	}

	if err != nil {
		printErr("error: %v\n", err)
		os.Exit(exitIOErr)
	}

	os.Exit(loop(b))
}

func createI2CBus() (zbus.Bus, error) {
	if len(os.Args) != 4 {
		printErr("error: invalid 'i2c' bus arguments")
		os.Exit(exitUsage)
	}

	dev, err := strconv.Atoi(os.Args[2])
	if err != nil {
		printErr("error: invalid I2C device number\n")
		os.Exit(exitUsage)
	}

	pin, err := strconv.Atoi(os.Args[3])
	if err != nil {
		printErr("error: invalid GPIO pin number\n")
		os.Exit(exitUsage)
	}

	return zbus.NewI2CBus(dev, pin)
}

func createSimBus() (zbus.Bus, error) {
	if len(os.Args) != 3 {
		printErr("error: invalid 'i2c' bus arguments")
		os.Exit(exitUsage)
	}

	return zbus.NewSimBus(os.Args[2])
}

func printErr(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

func printHelp() {
	printErr(`ZEN-bus master

This program provides master-side implementation of the ZEN-bus protocol.
Hardware ZEN-bus is supported only on Linux and is implemented using I²C
and GPIO. For other platforms and testing/debugging there is a "simulated"
bus based on a simple binary TCP protocol. ZEN module firmware will use
this bus simulation when running on native POSIX platform.

To create an I²C Zbus master, run

  zbus i2c <i2c_num> <gpio_num>

where <i2c_num> is the number of the I²C device (/dev/i2c-X) and <gpio_num>
is the number of the GPIO pin (/sys/class/gpio/gpioX). 

To create a simulated Zbus master, run

  zbus sim <address>

where <address> is the address in "host:port" format the TCP server will
bind to. The server will bind to all available interfaces if the "host" part
is empty. Some examples: ":7802", "[::1]:7802"
`)
}

func loop(b zbus.Bus) int {
	defer b.Close()

	done := make(chan struct{})

	go func() {
		sig := make(chan os.Signal, 8)
		signal.Notify(sig, os.Interrupt)

		<-sig

		close(done)
	}()

	proto := NewTextProtocol(os.Stdin, os.Stdout)
	input := readCommands(proto)

	proto.WriteVersion(zbus.Version)

	var err error

loop:
	for err = error(nil); err == nil; {
		select {
		case in, ok := <-input:
			if in.err != nil || !ok {
				err = in.err
			} else {
				err = processCommand(b, in.cmd)
			}

		case ev, ok := <-b.Events():
			if !ok {
				err = nil
			} else {
				err = processEvent(proto, ev)
			}

		case <-done:
			break loop
		}
	}

	if err != nil && err != io.EOF {
		printErr("error: %v\n", err)
		return exitIOErr
	}

	return 0
}

func readCommands(proto Protocol) chan input {
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

func processCommand(b zbus.Bus, cmd Command) error {
	switch cmd.Type {
	case CmdReset:
		b.Reset()

	case CmdPacket:
		b.Send(cmd.Pkt)
	}
	return nil
}

func processEvent(p Protocol, ev zbus.Event) error {
	switch ev.Type {
	case zbus.ResetEvent:
		p.WriteReset()

	case zbus.PacketEvent:
		p.WritePacket(*ev.Pkt)

	case zbus.ErrorEvent:
		if ev.Err == zbus.SysError {
			// TODO(mbenda): proper SysError error passing
			return errors.New("unrecoverable bus error")
		}
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
