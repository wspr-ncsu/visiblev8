package core

import (
	"io"
	"log"
	"os"
)

// GetEnvDefault gets the named ENV var _or_ a default value if it's not there
func GetEnvDefault(key, def string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		val = def
	}
	return val
}

// ClosingReader attempts to close the underlying reader on EOF
type ClosingReader struct {
	reader io.Reader
}

func (cr *ClosingReader) Read(p []byte) (int, error) {
	n, err := cr.reader.Read(p)
	if err != nil {
		closer, ok := cr.reader.(io.Closer)
		if ok {
			cerr := closer.Close()
			if cerr != nil {
				// Log the failure but do not die/panic--continue back to caller
				log.Print(cerr)
			}
		}
	}
	return n, err
}

// NewClosingReader wraps an existing io.Reader to auto-close on EOF
func NewClosingReader(rawReader io.Reader) *ClosingReader {
	return &ClosingReader{reader: rawReader}
}

// StripCurlies removes a bracketing '{' '}' pair of characters if present
func StripCurlies(s string) (string, bool) {
	if len(s) >= 2 {
		if (s[0] == '{') && (s[len(s)-1] == '}') {
			return s[1 : len(s)-1], true
		}
	}
	return s, false
}

// StripQuotes removes a bracketing '"' '"' pair of characters if present
func StripQuotes(s string) (string, bool) {
	if len(s) >= 2 {
		if (s[0] == '"') && (s[len(s)-1] == '"') {
			return s[1 : len(s)-1], true
		}
	}
	return s, false
}
