/*
 * Copyright (c) 2018 omSquare s.r.o.
 *
 * SPDX-License-Identifier: Apache-2.0 
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <errno.h>
#include <unistd.h>

#include "cmd.h"

static const char PACKET[] = "PKT %02x %02x\n";
static const char ACK[] = "ACK %02x\n";
static const char ERROR[] = "ERR %02x\n";
static const char CONNECT[] = "CONN %02x\n";
static const char DISCONNECT[] = "DISC %02x\n";

static const char HEX[] = "01234567890ABCDEF";

static const int LINE_LEN = 32;

static int cmd_in = STDIN_FILENO;
static int cmd_out = STDOUT_FILENO;

static char write_buf[81];
static char read_buf[81];
static int read_len;
static int read_pos;

int cmd_write_packet(int addr, struct cmd_packet pkt)
{
    // write header
    snprintf(write_buf, sizeof(write_buf), PACKET, addr, pkt.len);
    if (write(cmd_out, write_buf, strlen(write_buf)) <= 0) {
        return -1;
    }

    // write packet data
    for (int i = 0; i < pkt.len; i += LINE_LEN) {
        int limit = i + LINE_LEN;
        if (limit > pkt.len) {
            limit = pkt.len;
        }

        size_t pos = 0;
        for (int j = i; j < limit; j++) {
            int byte = pkt.data[j];
            write_buf[pos++] = HEX[byte % 16];
            write_buf[pos++] = HEX[byte / 16];
        }

        write_buf[pos++] = '\n';

        if (write(cmd_out, write_buf, pos) <= 0) {
            return -1;
        }
    }

    return 0;
}

int cmd_write_ack(int addr)
{
    snprintf(write_buf, sizeof(write_buf), ACK, addr);
    if (write(cmd_out, write_buf, strlen(write_buf)) <= 0) {
        return -1;
    }

    return 0;
}

static int cmd_write_error(int addr)
{
    snprintf(write_buf, sizeof(write_buf), ERROR, addr);
    if (write(cmd_out, write_buf, strlen(write_buf)) <= 0) {
        return -1;
    }

    return 0;
}

static int cmd_write_connect(const struct cmd *cmd)
{
    // TODO mbenda: UDID
    snprintf(write_buf, sizeof(write_buf), CONNECT, cmd->addr);
    if (write(cmd_out, write_buf, strlen(write_buf)) <= 0) {
        return -1;
    }

    return 0;
}

static int cmd_write_disconnect(int addr)
{
    snprintf(write_buf, sizeof(write_buf), DISCONNECT, addr);
    if (write(cmd_out, write_buf, strlen(write_buf)) <= 0) {
        return -1;
    }

    return 0;
}

int cmd_write(const struct cmd *cmd)
{
    switch (cmd->code) {
    case CMD_PACKET:
        return cmd_write_packet(cmd->addr, *cmd->pkt);

    case CMD_ACK:
        return cmd_write_ack(cmd->addr);

    case CMD_ERROR:
        return cmd_write_error(cmd->addr);

    case CMD_CONNECT:
        return cmd_write_connect(cmd);

    case CMD_DISCONNECT:
        return cmd_write_disconnect(cmd->addr);

    default:
        errno = EINVAL;
        return -1;
    }
}

static int cmd_read_token(int fd, char *token, int size)
{
    int pos = 0;

    do {
        // skip leading whitespace
        while (read_pos < read_len && isspace(read_buf[read_pos])) {
            read_pos++;
        }

        // read token characters
        while (read_pos < read_len && !isspace(read_buf[read_pos])
                && pos < size) {
            token[pos++] = read_buf[read_pos++];
        }

        // re-read buffer if needed
        if (read_pos == read_len) {
            ssize_t len = read(fd, read_buf, sizeof(read_buf));
            if (len < 0) {
                return -1;
            }

            read_pos = 0;
            read_len = (int) len;
        }
    } while (pos == 0 && read_len > 0);

    return pos;
}

int cmd_read(int fd, struct cmd *cmd)
{
    char token[16];

    // read command
    int len = cmd_read_token(fd, token, sizeof(token) - 1);

    if (len < 0) {
        return -1;
    }

    if (len == 0) {
        // EOF
        return 1;
    }

    token[len] = '\0';

    if (strcmp(token, "RST") == 0) {
        *cmd = (struct cmd) {
                .code = CMD_RESET,
                .addr = 0,
                .pkt = NULL,
                .udid = NULL
        };
        return 0;
    }

    // TODO mbenda: PKT

    errno = EPROTO;
    return -1;
}

void cmd_free(struct cmd *cmd)
{
    if (cmd->pkt) {
        free(cmd->pkt);
        cmd->pkt = NULL;
    }

    if (cmd->udid) {
        free(cmd->udid);
        cmd->udid = NULL;
    }
}
