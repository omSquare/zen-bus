/*
 * Copyright (c) 2018 omSquare s.r.o.
 *
 * SPDX-License-Identifier: Apache-2.0 
 */

#pragma once

/**
 * Initializes alert GPIO pin a returns a file descriptor for edge polling.
 *
 * @param gpio_num GPIO pin number
 * @return pin file descriptor, or -1 if error
 */
int alert_open(int gpio_num);

/**
 * Reads the current alert signal value.
 *
 * @return alert signal level, or -1 if error
 */
int alert_value(int fd);
