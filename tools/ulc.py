#!/usr/bin/env python3
import os
import sys
import socket
from contextlib import closing

from uls import LENGTH_FIELD

CONN_HOST = os.environ.get("CONN_HOST", "127.0.0.1")
CONN_PORT = int(os.environ.get("CONN_PORT", 52528))


def main(argv):
    client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    client.connect((CONN_HOST, CONN_PORT))
    with closing(client):
        filename = f"logfile.{os.getpid()}.log"
        print(f"logging to '{filename}'")
        msg = LENGTH_FIELD.pack(len(filename)) + filename.encode("utf8")
        client.sendall(msg)

        while True:
            hunk = sys.stdin.read(4096)
            if not hunk:
                break
            print(f"sending '{repr(hunk)}'")
            client.sendall(hunk.encode("utf8"))


if __name__ == "__main__":
    main(sys.argv)
