#include <sys/socket.h>
#include <arpa/inet.h>

#define NOB_IMPLEMENTATION
#define NOB_STRIP_PREFIX
#include "nob.h"
#define FLAG_IMPLEMENTATION
#include "flag.h"

void usage(void)
{
    fprintf(stderr, "Usage: %s [OPTIONS]\n", flag_program_name());
    fprintf(stderr, "OPTIONS:\n");
    flag_print_options(stderr);
}

int main(int argc, char **argv)
{
    char **address = flag_str("a", "127.0.0.1", "Address of the server");
    size_t *port = flag_size("p", 8080, "Port of the server");
    bool *help = flag_bool("help", false, "Print this help message");

    if (!flag_parse(argc, argv)) {
        usage();
        flag_print_error(stderr);
        return 1;
    }

    if (*help) {
        usage();
        return 0;
    }

    if (*address == NULL) {
        usage();
        fprintf(stderr, "ERROR: no value provided -%s\n", flag_name(address));
        return 1;
    }

    int client = socket(AF_INET, SOCK_STREAM, 0);
    if (client < 0) {
        fprintf(stderr, "ERROR: could not create socket: %s\n", strerror(errno));
        return 1;
    }

    // Set server address
    struct sockaddr_in server_address = {0};
    server_address.sin_family = AF_INET;
    server_address.sin_port = htons(*port);
    inet_pton(AF_INET, *address, &server_address.sin_addr);

    if (connect(client, (const struct sockaddr*)&server_address, sizeof(server_address)) < 0) {
        fprintf(stderr, "ERROR: could not connect to %s: %s\n", *address, strerror(errno));
        return 1;
    }

    // NOTE: This leaks a bit of memory in the child process.
    // But do we actually care? It's a one off leak anyway...
    Cmd cmd_null = {0};
    cmd_append(&cmd_null, "mpv", temp_sprintf("--input-ipc-client=fd://%d", client));
    da_append_many(&cmd_null, flag_rest_argv(), flag_rest_argc());
    cmd_append(&cmd_null, NULL);

    if (execvp(cmd_null.items[0], (char * const*) cmd_null.items) < 0) {
        nob_log(ERROR, "Could not exec child process for %s: %s", cmd_null.items[0], strerror(errno));
        return 1;
    }

    UNREACHABLE("mpv-client");
}
