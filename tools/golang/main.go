package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

func handleStream(conn net.Conn, logTimeout time.Duration) {
	defer conn.Close()
	addr := conn.RemoteAddr().String()

	if err := conn.SetReadDeadline(time.Now().Add(logTimeout)); err != nil {
		panic(fmt.Errorf("%s: unable to set read timeout: %w", addr, err))
	}
	var filenameLength uint32
	if err := binary.Read(conn, binary.BigEndian, &filenameLength); err != nil {
		panic(fmt.Errorf("%s: unable to read header length: %w", addr, err))
	}
	rawFilename := make([]byte, filenameLength)
	if _, err := io.ReadFull(conn, rawFilename); err != nil {
		panic(fmt.Errorf("%s: unable to read filename: %w", addr, err))
	}

	filename := filepath.Base(string(rawFilename))
	fd, err := os.Create(filename)
	if err != nil {
		panic(fmt.Errorf("%s: unable to open file '%s' for writing: %w", addr, filename, err))
	}
	defer fd.Close()
	log.Printf("%s: logging to '%s'\n", addr, filename)

	var buffer [4096]byte
	for {
		n, readErr := conn.Read(buffer[:])
		for sofar := 0; sofar < n; {
			m, writeErr := fd.Write(buffer[sofar:n])
			if writeErr != nil {
				panic(fmt.Errorf("%s: failed to write to file %w", addr, writeErr))
			}
			sofar += m
		}
		if readErr != nil {
			if readErr != io.EOF {
				panic(fmt.Errorf("%s: failed to read from socket %w", addr, readErr))
			} else {
				log.Printf("%s: EOF on socket, closing down\n", addr)
				return
			}
		}
	}
}

func main() {
	var bindAddr string
	var logTimeoutSec int

	flag.StringVar(&bindAddr, "bindAddr", "127.0.0.1:52528", "Bind/receive on this TCP endpoint")
	flag.IntVar(&logTimeoutSec, "logTimeout", 300, "Timeout/close log streams after this many seconds of inactivity")
	flag.Parse()
	logTimeoutDuration := time.Duration(logTimeoutSec) * time.Second

	server, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("unable to bind/listen on %s: %v\n", bindAddr, err)
	}
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatalf("unable to accept connection: %v\n", err)
		}
		go handleStream(conn, logTimeoutDuration)
	}
}
