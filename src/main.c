#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sysexits.h>
#include <sys/poll.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>

#include "alert.h"
#include "cmd.h"

#define FD_STDIN 0
#define FD_ALERT 1

static int alert_fd = -1;

static void cleanup(void)
{
    if (alert_fd >= 0) {
        close(alert_fd);
    }
}

static int parse_cmdline(int *i2c_num, int *gpio_num, int argc, char *argv[])
{
    if (argc != 3) {
        goto usage;
    }

    char *end;

    *i2c_num = (int) strtol(argv[1], &end, 10);
    if (end - argv[1] != strlen(argv[1]) || *i2c_num < 0 || *i2c_num > 9) {
        fprintf(stderr, "error: invalid i2c_num");
        goto usage;
    }

    *gpio_num = (int) strtol(argv[2], &end, 10);
    if (end - argv[2] != strlen(argv[2]) || *gpio_num < 0 || *gpio_num > 9999) {
        fprintf(stderr, "error: invalid gpio_num");
        goto usage;
    }

    return 0;

    usage:
    fprintf(stderr, "usage: %s <i2c_num> <gpio_num>\n", argv[0]);
    return -1;
}

static int process_cmd(void)
{
    struct cmd cmd;

    int result = cmd_read(STDIN_FILENO, &cmd);
    if (result < 0) {
        cmd_free(&cmd);

        if (errno == EWOULDBLOCK) {
            fprintf(stderr, "BLOCK!\n");
            return 0;
        }

        perror("error: stdin");
        exit(EX_IOERR);
    }

    if (result == 1) {
        // end of STDIN
        cmd_free(&cmd);
        exit(EXIT_SUCCESS);
    }

    fprintf(stderr, "command: %d\n", cmd.cmd);
    return 1;
}

static int process_alert()
{
    int alert = alert_value(alert_fd);
    if (alert < 0) {
        return -1;
    }

    if (alert == 0) {
        // TODO poll I2C
    }

    return 0;
}

int main(int argc, char *argv[])
{
    // parse command line arguments
    int i2c_num, gpio_num;
    if (parse_cmdline(&i2c_num, &gpio_num, argc, argv)) {
        exit(EX_USAGE);
    }

    // initialize devices
    alert_fd = alert_open(gpio_num);
    if (alert_fd < 0) {
        perror("error: alert");
        exit(EX_NOINPUT);
    }

    // TODO mbenda: I2C

    // preprare file descriptors for polling
    fcntl(STDIN_FILENO, F_SETFL, fcntl(STDIN_FILENO, F_GETFL, 0) | O_NONBLOCK);

    struct pollfd fds[2];
    fds[FD_STDIN].fd = STDIN_FILENO;
    fds[FD_STDIN].events = POLLIN;
    fds[FD_ALERT].fd = alert_fd;
    fds[FD_ALERT].events = POLLOUT;

    // enter command loop
    while (poll(fds, 2, -1) >= 0) {
        // check for command input
        if (fds[FD_STDIN].revents) {
            while (process_cmd()) {
                // process commands
            }
        }

        // check for alert
        if (fds[FD_ALERT].revents) {
            if (process_alert()) {
                perror("error: alert");
                exit(EX_IOERR);
            }
        }
    }

    return 0;
}
