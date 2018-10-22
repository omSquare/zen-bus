/*
 * Copyright (c) 2018 omSquare s.r.o.
 *
 * SPDX-License-Identifier: Apache-2.0 
 */

#include "alert.h"

#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

static int gpio_open(int gpio_num, const char *attr)
{
    // format file name
    char name[40];
    snprintf(name, sizeof(name), "/sys/class/gpio/gpio%d/%s", gpio_num, attr);

    return open(name, O_RDWR);
}

static int gpio_write(int gpio_num, const char *attr, const char *value)
{
    int fd = gpio_open(gpio_num, attr);
    if (fd < 0) {
        return -1;
    }

    ssize_t n = write(fd, value, strlen(value));
    close(fd);

    return n > 0 ? 0 : -1;
}

int alert_open(int gpio_num)
{
    // set direction and edge
    if (gpio_write(gpio_num, "direction", "in")) {
        // TODO ignore?
        return -1;
    }

    if (gpio_write(gpio_num, "edge", "both")) {
        return -1;
    }

    // open the "value" file to get the initial value and later us it for
    // interrupt detection
    return gpio_open(gpio_num, "value");
}

int alert_value(int fd)
{
    if (lseek(fd, 0, SEEK_SET) < 0) {
        return -1;
    }

    char str[8];
    memset(str, 0, sizeof(str));
    if (read(fd, str, sizeof(str) - 1) < 0) {
        return -1;
    }

    return strcmp(str, "0");
}
