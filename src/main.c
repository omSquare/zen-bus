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

static struct pollfd fds[2];

#define FD_STDIN 0
#define FD_ALERT 1

void cleanup(void)
{
    alert_close();
}

static int parse_cmdline(int *i2c_num, int *gpio_num, int argc, char *argv[])
{
    // TODO mbenda: implement this
    return 0;
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

        perror("error");
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

int main(int argc, char *argv[])
{
    // parse command line arguments
    int i2c_num, gpio_num;
    if (parse_cmdline(&i2c_num, &gpio_num, argc, argv)) {
        exit(EX_USAGE);
    }

    // initialize devices

    // preprare file descriptors for polling
    fcntl(STDIN_FILENO, F_SETFL, fcntl(STDIN_FILENO, F_GETFL, 0) | O_NONBLOCK);

    fds[FD_STDIN].fd = STDIN_FILENO;
    fds[FD_STDIN].events = POLLIN;
    fds[FD_ALERT].fd = STDOUT_FILENO;
    fds[FD_ALERT].events = POLLOUT;

    // enter command loop
    while (poll(fds, 1, -1) >= 0) { // FIXME 2 fds
        // check for command input
        if (fds[FD_STDIN].revents) {
            while (process_cmd()) {
                // process commands
            }
        }

        // check for alert

    }

    return 0;
}
