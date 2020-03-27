#include <stdio.h>
#include <string.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <netinet/in.h>
#include <unistd.h>

int sendall(int sock, const char *data, size_t dlen)
{
    int ret = -1;
    size_t sofar = 0;
    while (sofar < dlen)
    {
        ssize_t sent = send(sock, &data[sofar], dlen - sofar, 0);
        if (sent < 0)
        {
            goto cleanup;
        }
        sofar += sent;
    }
    ret = 0;
cleanup:
    return ret;
}

int main()
{
    int ret = 1;
    int sock = -1;
    struct sockaddr_in sa = {
        .sin_family = AF_INET,
        .sin_port = htons(52528),
        .sin_addr = {
            .s_addr = inet_addr("127.0.0.1")}};

    if ((sock = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP)) < 0)
    {
        perror("socket");
        goto cleanup;
    }
    if (connect(sock, (const struct sockaddr *)&sa, sizeof sa))
    {
        perror("connect");
        goto cleanup;
    }
    if (shutdown(sock, SHUT_RD))
    {
        perror("shutdown");
        goto cleanup;
    }

    const char *filename = "logfile.test.log";
    uint32_t hdr = htonl(strlen(filename));
    if (sendall(sock, (const char *)&hdr, sizeof hdr))
    {
        perror("sendall");
        goto cleanup;
    }
    if (sendall(sock, filename, strlen(filename)))
    {
        perror("sendall");
        goto cleanup;
    }

    const char *message = "hello, world!\nthis is a log\n\nwith lines\n\nand\nstuff...\n";
    if (sendall(sock, message, strlen(message)))
    {
        perror("sendall");
        goto cleanup;
    }

    ret = 0;
cleanup:
    if (sock >= 0)
    {
        if (shutdown(sock, SHUT_WR))
        {
            perror("shutdown");
        }
        if (close(sock))
        {
            perror("close");
        }
    }
    return ret;
}