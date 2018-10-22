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

static int alert_fd = -1;
static int alert_pin;

static int gpio_open(const char *attr)
{
    // format file name
    char name[40];
    snprintf(name, sizeof(name), "/sys/class/gpio%d/%s", alert_pin, attr);

    return open(name, O_RDWR);
}

static int gpio_write(const char *attr, const char *value)
{
    int fd = gpio_open(attr);
    if (fd < 0) {
        return -1;
    }

    ssize_t n = write(fd, value, strlen(value));
    close(fd);

    return n > 0 ? 0 : -1;
}

int alert_open(int gpio_num)
{
    if (alert_fd >= 0) {
        // already open
        errno = EBUSY;
        return -1;
    }

    if (gpio_num < 0 || gpio_num > 999) {
        // sanity check failed
        errno = EINVAL;
        return -1;
    }

    alert_pin = gpio_num;

    // set direction and edge
    if (gpio_write("direction", "in")) {
        // TODO ignore?
        return -1;
    }

    if (gpio_write("edge", "both")) {
        return -1;
    }

    // open the "value" file to get the initial value and later us it for
    // interrupt detection
    alert_fd = gpio_open("value");
    if (alert_fd < 0) {
        return -1;
    }

    return alert_fd;
}

int alert_close(void)
{
    // close the file descriptor...
    int ret = close(alert_fd);

    alert_fd = -1;
    return ret;
}

int alert_value(void)
{
    if (alert_fd < 0) {
        return -1;
    }

    if (lseek(alert_fd, 0, SEEK_SET) < 0) {
        return -1;
    }

    char str[8];
    memset(str, 0, sizeof(str));
    if (read(alert_fd, str, sizeof(str) - 1) < 0) {
        return -1;
    }

    return strcmp(str, "0");
}
