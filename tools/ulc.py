#!/usr/bin/env python3
import os
import sys
import socket

from uls import (
    MSG_HEADER,
    CMD_FLAG_OPEN,
    CMD_FLAG_WRITE,
    CMD_FLAG_FLUSH,
    CMD_FLAG_CLOSE,
)

CONN_HOST = os.environ.get("CONN_HOST", "127.0.0.1")
CONN_PORT = int(os.environ.get("CONN_PORT", 52528))
CHUNK_SIZE = int(os.environ.get("CHUNK_SIZE", 65536))


def main(argv):
    client = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    client.connect((CONN_HOST, CONN_PORT))

    cookie = os.getpid()
    filename = f"logfile.{cookie}.log"

    print(f"logging to '{filename}'")
    msg = MSG_HEADER.pack(cookie, CMD_FLAG_OPEN) + filename.encode("utf8")
    client.sendmsg([msg])

    sofar = 0
    while True:
        hunk = sys.stdin.read(4096)
        if not hunk:
            break

        print(f"sending '{repr(hunk)}'")
        cmd = CMD_FLAG_WRITE
        raw_hunk = hunk.encode("utf8")
        sofar += len(raw_hunk)
        if sofar >= CHUNK_SIZE:
            cmd |= CMD_FLAG_FLUSH
            sofar = 0
        msg = MSG_HEADER.pack(cookie, cmd) + raw_hunk
        client.sendmsg([msg])

    msg = MSG_HEADER.pack(cookie, CMD_FLAG_WRITE | CMD_FLAG_CLOSE) + b"---\nbye\n"
    client.sendmsg([msg])


if __name__ == "__main__":
    main(sys.argv)
