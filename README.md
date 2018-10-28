# zen-bus

[![Build Status](https://travis-ci.org/omSquare/zen-bus.svg?branch=master)](https://travis-ci.org/omSquare/zen-bus)
[![Go Report Card](https://goreportcard.com/badge/github.com/omSquare/zen-bus)](https://goreportcard.com/report/github.com/omSquare/zen-bus)
[![GoDoc](https://godoc.org/github.com/omSquare/zen-bus?status.svg)](https://godoc.org/github.com/omSquare/zen-bus)
[![apache license](https://img.shields.io/badge/license-Apache-blue.svg)](LICENSE)


ZEN-bus (Zbus) is  an I²C based bus for reliable local communication between ZEN gateways/bridges and ZEN
I/O devices. This project implements the master-side of the bus on Linux using its userspace support fo GPIO and I²C.

### Requirements

Zbus compiles only with `GOOS=linux`.

### References

* https://www.kernel.org/doc/Documentation/i2c/dev-interface
* https://www.kernel.org/doc/Documentation/gpio/sysfs.txt
