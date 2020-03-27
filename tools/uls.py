#!/usr/bin/env python3
import logging
import os
import socket
import select
import sys
import struct
import time


BIND_HOST = os.environ.get("BIND_HOST", "localhost")
BIND_PORT = int(os.environ.get("BIND_PORT", "52528"), 10)
LOG_TIMEOUT = int(os.environ.get("LOG_TIMEOUT", "300"), 10)

BUFSIZE = 64 * 1024
MSG_HEADER = struct.Struct("<IB")
CMD_FLAG_OPEN = 0x01
CMD_FLAG_WRITE = 0x02
CMD_FLAG_FLUSH = 0x04
CMD_FLAG_CLOSE = 0x08


def main(argv):
    logging.basicConfig(level=logging.INFO)
    log_map = {}
    log_tlu = {}

    serv = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    serv.bind((BIND_HOST, BIND_PORT))

    while True:
        rsel, _, xsel = select.select([serv], [], [serv], LOG_TIMEOUT)
        now = time.time()
        if xsel:
            logging.error(f"got error condition on {xsel}; aborting...")
            sys.exit(1)
        if rsel:
            data, _, _, addr = serv.recvmsg(BUFSIZE)
            try:
                header, body = data[: MSG_HEADER.size], data[MSG_HEADER.size :]
                cookie, cmd = MSG_HEADER.unpack(header)

                if cmd & CMD_FLAG_OPEN:
                    fd = log_map[cookie] = open(body, "ab")
                    logging.info(f"new cookie {cookie:#x} ({cmd:#b}): '{body}'")
                else:
                    fd = log_map.get(cookie)
                    if not fd:
                        logging.error(
                            f"access {cmd:#b} to unknown cookied {cookie:#x} from {addr} (ignoring)"
                        )
                        continue
                log_tlu[cookie] = now

                if cmd & CMD_FLAG_WRITE:
                    fd.write(body)

                if cmd & CMD_FLAG_FLUSH:
                    fd.flush()

                if cmd & CMD_FLAG_CLOSE:
                    fd.close()
                    del log_map[cookie]
            except Exception:
                logging.exception(f"error processing message from {addr} (ignoring)")

        timeouts = [
            cookie
            for cookie, last_time in log_tlu.items()
            if now - last_time >= LOG_TIMEOUT
        ]
        for cookie in timeouts:
            logging.warning(
                f"log for cookie {cookie:#x} has seen no activity in at least {LOG_TIMEOUT} seconds; closing..."
            )
            log_map.pop(cookie).close()
            del log_tlu[cookie]


if __name__ == "__main__":
    main(sys.argv)
