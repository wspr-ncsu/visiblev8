package core

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

// AnnotateStream is a simplified version of the core parser that just annotates raw lines
func AnnotateStream(stream io.Reader, aggCtx *AggregationContext) error {
	ln := NewLogInfo(aggCtx.LogOid, aggCtx.RootName, aggCtx.SubmissionID)

	// Read lines from input
	scan := bufio.NewScanner(stream)

	// Support LOOOONG lines
	scan.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), 128*1024*1024)

	// Prepare for JSON output to stdout (no options on that)
	jstream := json.NewEncoder(os.Stdout)

	// Start processing log lines
	var lineCount int
	for scan.Scan() {
		line := scan.Bytes()
		lineCount++
		doc := JSONObject{
			"t": string(line),
		}
		if len(line) > 0 {
			code := line[0]
			fields := splitFields(line[1:])
			switch code {
			case '~':
				ln.changeIsolate(fields[0])
			case '$':
				scriptID, err := strconv.Atoi(fields[0])
				if err != nil {
					return err
				}
				script := ln.addScript(scriptID, fields[1], fields[2])
				doc["d"] = scriptID
				doc["s"] = hex.EncodeToString(script.CodeHash.SHA2[:]) // TODO: move away from SHA2-256-only script hashes to ID stuff
			case '!':
				scriptID, err := strconv.Atoi(fields[0])
				if err != nil {
					ln.resetContext()
				} else {
					ln.changeScript(scriptID)
				}
			case '@':
				originString, _ := StripQuotes(fields[0])
				originSecurityToken, _ := StripQuotes(fields[1])
				ln.changeOrigin(originString, originSecurityToken)
			default:
				offset, err := strconv.Atoi(fields[0])
				if err != nil {
					return fmt.Errorf("%d: invalid script offset '%s'", lineCount, fields[0])
				} else if offset >= 0 && ln.World.Context.Script != nil {
					doc["o"] = offset
				}
			}
			if ln.World.Context.Script != nil {
				doc["x"] = ln.World.Context.Script.ID
			}
		}
		jstream.Encode(doc)
	}
	if scan.Err() != nil {
		return scan.Err()
	}

	return nil
}
