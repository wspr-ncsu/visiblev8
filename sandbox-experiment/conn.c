#include <sys/types.h>
#include <sys/socket.h>
#include <netdb.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <string.h>

static const char *ENV_VV8_LOG_HOST = "VV8_LOG_HOST";
static const char *DEFAULT_VV8_LOG_HOST = "localhost";
static const char *ENV_VV8_LOG_PORT = "VV8_LOG_PORT";
static const char *DEFAULT_VV8_LOG_PORT = "5580";

int
connect_to_vv8_server() {
	int ret = -1;
	const char *log_host = NULL;
	const char *log_port = NULL;
	struct addrinfo hints;
	struct addrinfo *result = NULL, *rp;
	int sfd, s;
	size_t len;
	ssize_t nread;

	memset(&hints, 0, sizeof(hints));
	hints.ai_family = AF_UNSPEC;
	hints.ai_socktype = SOCK_STREAM;
	hints.ai_flags = 0;
	hints.ai_protocol = 0;

	if ((log_host = getenv(ENV_VV8_LOG_HOST)) == NULL) {
		log_host = DEFAULT_VV8_LOG_HOST;	
	}
	if ((log_port = getenv(ENV_VV8_LOG_PORT)) == NULL) {
		log_port = DEFAULT_VV8_LOG_PORT;
	}
	printf("debug: connecting to %s:%s\n", log_host, log_port);

	s = getaddrinfo(log_host, log_port, &hints, &result);
	if (s != 0) {
		fprintf(stderr, "getaddrinfo: %s\n", gai_strerror(s));
		goto cleanup;
	}
	for (rp = result; rp != NULL; rp = rp->ai_next) {
		sfd = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
		if (sfd < 0) continue;
		if (connect(sfd, rp->ai_addr, rp->ai_addrlen) != -1) break;
		close(sfd);
	}
	if (rp == NULL) {
		fprintf(stderr, "could not connect to %s:%s\n", log_host, log_port);
		goto cleanup;
	}

	ret = sfd;
cleanup:
	if (result != NULL) {
		freeaddrinfo(result);
	}
	return ret;
}


int
main(int argc, char *argv[]) {
	int ret = EXIT_FAILURE;
	int sock = -1;
	char buf[512], *bp;
	size_t left;
	ssize_t sent;

	if ((sock = connect_to_vv8_server()) < 0) {
		fprintf(stderr, "unable to connect to VV8 log server\n");
		goto cleanup;
	}

	snprintf(buf, sizeof(buf), "hello from pid=%d\n", getpid());
	left = strlen(buf);
	bp = buf;
	while (left > 0) {
		if ((sent = send(sock, bp, left, 0)) < 0) {
			perror("send");
			goto cleanup;
		}
		left -= sent;
		bp += sent;
	}

	ret = EXIT_SUCCESS;
cleanup:
	if (sock >= 0) {
		if (shutdown(sock, SHUT_RDWR)) perror("shutdown");
		if (close(sock)) perror("close");
		sock = -1;
	}
	return ret;
}
