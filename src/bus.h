/*
 * Copyright (c) 2018 omSquare s.r.o.
 *
 * SPDX-License-Identifier: Apache-2.0 
 */

#pragma once

#define BUS_MAX_PACKET 255

int bus_open(int i2c_num);

int bus_reset(int fd);

int bus_send(int fd, int addr, void *data, int len);

int bus_poll(int fd);

int bus_discover(int fd);
