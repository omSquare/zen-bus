/*
 * Copyright (c) 2018 omSquare s.r.o.
 *
 * SPDX-License-Identifier: Apache-2.0 
 */

#pragma once

#include <stdio.h>
#include <stdint.h>

enum {
    CMD_RESET,
    CMD_PACKET,
    CMD_ACK,
    CMD_ERROR,
    CMD_CONNECT,
    CMD_DISCONNECT,
};

struct cmd {
    int cmd;
    int addr;
    struct cmd_packet *pkt;
    struct cmd_udid *udid;
};

struct cmd_packet {
    int len;
    uint8_t data[];
};

struct cmd_udid {
    // TODO mbenda: UDID structure
};

/**
 * Reads a command from the provided file descriptor. The command is stored in
 * the provided cmd structure.
 *
 * @param fd the input file descriptor
 * @param cmd the command destination
 * @return 0 on success, 1 if more data needed, -1 on error
 */
int cmd_read(int fd, struct cmd *cmd);

/**
 * Writes the given command to the provided file descriptor.
 *
 * @param fd the output file descriptor
 * @param cmd the command to write
 * @return 0 on success, 1 on EOF, -1 on error
 */
int cmd_write(int fd, const struct cmd *cmd);

/**
 * Releases allocated memory for the specified command (packet and UDID).
 *
 * @param cmd the command to clean up
 */
void cmd_free(struct cmd *cmd);
