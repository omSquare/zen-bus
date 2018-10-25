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
	"fmt"
	"io"
	"io/ioutil"
	"syscall"
)

type alert struct {
	state chan int // alert state
	err   error    // alert error (might be set when the state channel is closed)

	fd   int
	done chan struct{}
}

// Configures the alert pin by writing "in" to "direction" and "both" to "edge". Then opens the "value" file.
func newAlert(pin int) (*alert, error) {
	gpio := fmt.Sprintf("/sys/class/gpio/gpio%v/", pin)

	if err := ioutil.WriteFile(gpio+"direction", []byte("in"), 0666); err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(gpio+"edge", []byte("both"), 0666); err != nil {
		return nil, err
	}

	fd, err := syscall.Open(gpio+"value", syscall.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}

	// size of the done channel must be one to prevent deadlock between syscall.Select and selecting the done channel
	return &alert{make(chan int), nil, fd, make(chan struct{}, 1)}, nil
}

func (a *alert) close() {
	close(a.done)
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
}

// Listen for alert signal edges.
func (a *alert) watch() {
	defer syscall.Close(a.fd)
	defer close(a.state)

	buf := make([]byte, 16)

	for {
		select {
		case <-a.done:
			return

		default:
			break
		}

		if err := poll(a.fd); err != nil {
			a.err = err
			return
		}

		_, err := syscall.Seek(a.fd, 0, io.SeekStart)
		if err != nil {
			a.err = err
			return
		}

		n, err := syscall.Read(a.fd, buf)
		if err != nil {
			a.err = err
			return
		}

		if "0\n" == string(buf[:n]) {
			a.state <- 0
		} else {
			a.state <- 1
		}
	}
}

func poll(fd int) error {
	fds := &syscall.FdSet{}
	fds.Bits[fd/64] |= 1 << (uint(fd) % 64)
	_, err := syscall.Select(fd+1, nil, nil, fds, nil)

	if err == syscall.EINTR {
		return nil
	}

	return err
}
