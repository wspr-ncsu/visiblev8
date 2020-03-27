#!/usr/bin/env python3
import logging
import os
import select
import socket
import struct
import sys
import threading
import time
from contextlib import closing

BIND_HOST = os.environ.get("BIND_HOST", "localhost")
BIND_PORT = int(os.environ.get("BIND_PORT", "52528"), 10)
LOG_TIMEOUT = int(os.environ.get("LOG_TIMEOUT", "300"), 10)

LENGTH_FIELD = struct.Struct("!I")


def sock_read_all(conn, rlen) -> bytes:
    chunks = []
    recd = 0
    while recd < rlen:
        chunk = conn.recv(rlen - recd)
        chunks.append(chunk)
        recd += len(chunk)
    return b"".join(chunks)


def handle_connection(conn, addr):
    with closing(conn):
        (hlen,) = LENGTH_FIELD.unpack(sock_read_all(conn, LENGTH_FIELD.size))
        filename = sock_read_all(conn, hlen).decode("utf8")
        logging.info(f"{addr}: opening {filename}")
        with open(filename, "ab") as fd:
            while True:
                rset, _, xset = select.select([conn], [], [], LOG_TIMEOUT)
                if xset:
                    logging.error(f"{addr}: got error condition on {xsel}; aborting...")
                    return
                elif rset:
                    chunk = conn.recv(4096)
                    if not chunk:
                        logging.info(f"{addr}: end-of-stream")
                        return
                    fd.write(chunk)
                else:
                    logging.warning(f"{addr}: inactivity timeout")
                    return


def main(argv):
    logging.basicConfig(level=logging.INFO)
    log_map = {}
    log_tlu = {}

    serv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    serv.bind((BIND_HOST, BIND_PORT))

    serv.listen(5)
    while True:
        conn, addr = serv.accept()
        threading.Thread(
            target=handle_connection, args=(conn, addr), daemon=True
        ).start()


if __name__ == "__main__":
    main(sys.argv)
