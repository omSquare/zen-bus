/*
 * Copyright (c) 2018 omSquare s.r.o.
 *
 * SPDX-License-Identifier: Apache-2.0 
 */

#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/ioctl.h>

#include "cmd.h"

#if 0
#include <linux/i2c.h>
#include <linux/i2c-dev.h>
#else

#define I2C_RDWR 0x0707
#define I2C_FUNCS 0x0705
#define I2C_FUNC_I2C 0x00000001

typedef uint16_t __u16;
typedef uint8_t __u8;

struct i2c_msg {
    __u16 addr;                         /* slave address                      */
    __u16 flags;
#define I2C_M_TEN             0x0010    /* this is a ten bit chip address     */
#define I2C_M_RD              0x0001    /* read data, from slave to master    */
#define I2C_M_STOP            0x8000    /* if I2C_FUNC_PROTOCOL_MANGLING      */
#define I2C_M_NOSTART         0x4000    /* if I2C_FUNC_NOSTART                */
#define I2C_M_REV_DIR_ADDR    0x2000    /* if I2C_FUNC_PROTOCOL_MANGLING      */
#define I2C_M_IGNORE_NAK      0x1000    /* if I2C_FUNC_PROTOCOL_MANGLING      */
#define I2C_M_NO_RD_ACK       0x0800    /* if I2C_FUNC_PROTOCOL_MANGLING      */
#define I2C_M_RECV_LEN        0x0400    /* length will be first received byte */
    __u16 len;                          /* msg length                         */
    __u8 *buf;                          /* pointer to msg data                */
};
struct i2c_rdwr_ioctl_data {
    struct i2c_msg *msgs;  /* ptr to array of simple messages */
    int nmsgs;             /* number of messages to exchange */
};

#endif

#include "bus.h"

#define ADDR_CONF 0x76
#define ADDR_POLL 0x77

#define CMD_RESET 0x00

static uint8_t bus_buf[BUS_MAX_PACKET + 1];

int bus_open(int i2c_num)
{
    char file[16];

    snprintf(file, sizeof(file), "/dev/i2c-%d", i2c_num);
    int fd = open(file, O_RDWR);

    // check I2C functionality
    unsigned long funcs;
    if (ioctl(fd, I2C_FUNCS, &funcs) < 0) {
        return -1;
    }

    if ((funcs & I2C_FUNC_I2C) == 0) {
        close(fd);
        errno = ENODEV;
        return -1;
    }

    return fd;
}

int bus_reset(int fd)
{
    uint8_t data[] = {CMD_RESET};
    struct i2c_msg msg = {
            .addr = 0,
            .len = sizeof(data),
            .buf = data,
            .flags = 0,
    };

    // TODO bus error handling
    return ioctl(fd, I2C_RDWR, &(struct i2c_rdwr_ioctl_data) {&msg, 1});
}

int bus_send(int fd, int addr, void *data, int len)
{
    if (len < 1 || len > BUS_MAX_PACKET) {
        errno = EINVAL;
        return -1;
    }

    // TODO CRC
    bus_buf[0] = (uint8_t) len;
    memcpy(bus_buf + 1, data, len);

    struct i2c_msg msg = {
            .addr = (uint16_t) addr,
            .len = (uint16_t) len,
            .buf = bus_buf,
            .flags = 0,
    };

    // TODO bus error handling
    if (ioctl(fd, I2C_RDWR, &(struct i2c_rdwr_ioctl_data) {&msg, 1}) < 0) {
        return -1;
    }

    cmd_write_ack(addr);
}

int bus_poll(int fd)
{
    // perform poll transaction first
    struct i2c_msg msg = {
            .addr = ADDR_POLL,
            .len = 2,
            .buf = bus_buf,
            .flags = I2C_M_RD,
    };

    // TODO bus error handling
    if (ioctl(fd, I2C_RDWR, &(struct i2c_rdwr_ioctl_data) {&msg, 1}) < 0) {
        return -1;
    }

    // check received address and length
    uint16_t slave = bus_buf[0];
    uint16_t len = bus_buf[1];
    // TODO mbenda: implement this

    // read data from the slave
    msg.addr = slave;
    msg.len = len + (uint16_t) 1;

    // TODO bus error handling
    if (ioctl(fd, I2C_RDWR, &(struct i2c_rdwr_ioctl_data) {&msg, 1}) < 0) {
        return -1;
    }

    // TODO CRC

//    struct cmd_p
//    cmd_write_packet(slave, &(struct cmd_packet) {.len = len, .data = bus_buf});
    // TODO mbenda: ret

    return 0;
}
