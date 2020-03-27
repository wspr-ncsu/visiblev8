package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"log"
	"net"
	"os"
	"time"
)

type logStream struct {
	fd *os.File
	ts time.Time
}

type msgMode uint8

const (
	msgModeOpen msgMode = 1 << iota
	msgModeWrite
	msgModeFlush
	msgModeClose
)

type msgHeader struct {
	Cookie uint32  // Arbitrary stream ID handle
	Mode   msgMode // Bitmask used for operations
}

func main() {
	var bindAddr string
	var logTimeoutSec int

	flag.StringVar(&bindAddr, "bindAddr", "127.0.0.1:52528", "Bind/receive on this UDP endpoint")
	flag.IntVar(&logTimeoutSec, "logTimeout", 300, "Timeout/close log streams after this many seconds of inactivity")
	flag.Parse()
	logTimeoutDuration := time.Duration(logTimeoutSec) * time.Second

	server, err := net.ListenPacket("udp", bindAddr)
	if err != nil {
		log.Fatalf("unable to bind/listen on %s: %v\n", bindAddr, err)
	}
	rawMsg := make([]byte, 65536)
	logMap := make(map[uint32]*logStream)
	for {
		if err := server.SetDeadline(time.Now().Add(time.Duration(logTimeoutDuration))); err != nil {
			log.Fatalf("unable to set timeout on socket: %v\n", err)
		}
		recd, from, err := server.ReadFrom(rawMsg)
		now := time.Now()
		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				timeouts := make([]uint32, 0, len(logMap))
				for cookie, stream := range logMap {
					idleSec := now.Sub(stream.ts).Seconds()
					if idleSec >= float64(logTimeoutSec) {
						log.Printf("stream 0x%x has been idle for %f seconds and will be dropped", cookie, idleSec)
						timeouts = append(timeouts, cookie)
					}
				}
				for _, cookie := range timeouts {
					stream := logMap[cookie]
					if err := stream.fd.Close(); err != nil {
						log.Printf("close error for cookie 0x%x from %v: %v (ignoring)\n", cookie, from, err)
					}
					delete(logMap, cookie)
				}
			} else {
				log.Fatalf("error reading from socket: %v\n", err)
			}
		} else {
			msg := bytes.NewReader(rawMsg[:recd])
			var header msgHeader
			if err := binary.Read(msg, binary.LittleEndian, &header); err != nil {
				log.Printf("malformed message header from %v: %v (ignoring)\n", from, err)
				continue
			}
			body := rawMsg[binary.Size(header):recd]

			var stream *logStream
			if (header.Mode & msgModeOpen) == msgModeOpen {
				fd, err := os.Create(string(body))
				if err != nil {
					log.Fatalf("error opening file '%s' for cookie 0x%x: %v (ignoring)\n", body, header.Cookie, err)
					continue
				}
				stream = &logStream{fd, now}
				logMap[header.Cookie] = stream
			} else {
				var ok bool
				stream, ok = logMap[header.Cookie]
				if !ok {
					log.Printf("access (0x%x) to unknown cookie 0x%x from %v (ignoring)", header.Mode, header.Cookie, from)
					continue
				}
				stream.ts = now
			}

			if (header.Mode & msgModeWrite) == msgModeWrite {
				if _, err := stream.fd.Write(body); err != nil {
					log.Printf("write error for cookie 0x%x from %v: %v (ignoring)\n", header.Cookie, from, err)
					continue
				}
			}

			if (header.Mode & msgModeFlush) == msgModeFlush {
				if err := stream.fd.Sync(); err != nil {
					log.Printf("sync error for cookie 0x%x from %v: %v (ignoring)\n", header.Cookie, from, err)
					continue
				}
			}

			if (header.Mode & msgModeClose) == msgModeClose {
				if err := stream.fd.Close(); err != nil {
					log.Printf("close error for cookie 0x%x from %v: %v (ignoring)\n", header.Cookie, from, err)
				}
				delete(logMap, header.Cookie)
			}
		}
	}
}
