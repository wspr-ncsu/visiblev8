package core

// -------------------------------------------------------------------------------------
// generic vv8 log file parsing and context-awareness framework for aggregation
// -------------------------------------------------------------------------------------

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/sha3"
)

// Take a raw log string, expand all escape sequences, and split it into fields
func splitFields(line []byte) []string {
	allFields := make([]string, 0, 8)
	var curField strings.Builder
	var curDigs strings.Builder
	var surrogatePairFirst int

	type State int
	const (
		Copy  State = iota
		Copy2       // Special copy-char state indicating we JUST processed a ':' field separator
		Esc
		Hex
		Uni
	)

	state := Copy
	surrogatePairFirst = -1
	for _, c := range line {
		switch state {
		case Copy:
			if c == '\\' {
				state = Esc
			} else if c == ':' {
				allFields = append(allFields, curField.String())
				curField.Reset()
				state = Copy2
			} else {
				curField.WriteByte(c)
			}
		case Copy2:
			if c == '\\' {
				state = Esc
			} else if c == ':' {
				allFields = append(allFields, "")
			} else {
				curField.WriteByte(c)
				state = Copy
			}
		case Esc:
			if c == 'x' {
				state = Hex
				curDigs.Reset()
			} else if c == 'u' {
				state = Uni
				curDigs.Reset()
			} else {
				curField.WriteByte(c)
				state = Copy
			}
		case Hex:
			curDigs.WriteByte(c)
			if curDigs.Len() == 2 {
				code, _ := strconv.ParseUint(curDigs.String(), 16, 8)
				curField.WriteRune(rune(code))
				state = Copy
			}
		case Uni:
			curDigs.WriteByte(c)
			if curDigs.Len() == 4 {
				// A 16-bit Unicode codepoint--how hard could it be?
				rcode, _ := strconv.ParseUint(curDigs.String(), 16, 16)
				code := int(rcode)

				// Oh the joys of UTF16...
				if surrogatePairFirst >= 0 {
					code = (code - 0xdc00) + surrogatePairFirst + 0x10000
					surrogatePairFirst = -1
				}
				if (code >= 0xd800) && (code <= 0xdfff) {
					surrogatePairFirst = (code - 0xd800) * 0x400
				} else {
					curField.WriteRune(rune(code))
				}
				state = Copy
			}
		}
	}
	// Add on one last field if:
	// * there is trailing data (normal case)
	// * we ended on a ':' separator (corner case)
	if (curField.Len() > 0) || (state == Copy2) {
		allFields = append(allFields, curField.String())
	}

	return allFields
}

// FilterName identifies V8 object member names that should be filtered out of analysis
func FilterName(name string) bool {
	if name == "?" || name == "<anonymous>" {
		// Bogus/V8-noise/unusable; don't aggregate
		return true
	} else if _, err := strconv.ParseInt(name, 10, 64); err == nil {
		// Numeric property--do not aggregate
		return true
	} else {
		return false
	}
}

// InsertLogfile inserts (if not present) a record about this log file into PG
func (ln *LogInfo) InsertLogfile(sqldb *sql.DB) (int, error) {
	if !ln.Tabled {

		query := `INSERT INTO logfile
	(mongo_oid, uuid, root_name, size, lines, submissionid) VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT DO NOTHING`
		_, err := sqldb.Exec(query, ln.MongoID.String(), ln.ID.String(), ln.RootName, ln.Stats.Bytes, ln.Stats.Lines, ln.SubmissionID.String())

		if err != nil {
			return 0, err
		}

		ln.Tabled = true
	}

	var logID int

	err := sqldb.QueryRow(`SELECT id FROM logfile WHERE uuid = $1`, ln.ID.String()).Scan(&logID)
	if err != nil {
		return 0, err
	}

	return logID, nil
}

// NewLogInfo constructs a fresh LogInfo for the given vv8log Mongo oid (if available) and root log filename (if available)
func NewLogInfo(oid primitive.ObjectID, rootName string, submissionID uuid.UUID) *LogInfo {
	return &LogInfo{
		MongoID:      oid,
		SubmissionID: submissionID,
		ID:           uuid.New(),
		RootName:     rootName,
		Isolates:     make(map[string]*IsolateInfo),
	}
}

func (ln *LogInfo) changeIsolate(id string) *IsolateInfo {
	iso, ok := ln.Isolates[id]
	if !ok {
		iso = NewIsolateInfo(id)
		ln.Isolates[id] = iso
	}
	ln.World = iso
	ln.World.resetContext()
	return iso
}

func (ln *LogInfo) resetContext() {
	ln.World.resetContext()
}

func (ln *LogInfo) addScript(id int, src string, code string) *ScriptInfo {
	_, ok := ln.World.Scripts[id]

	if ok {
		panic(fmt.Errorf("redefining script ID %d in isolate %s", id, ln.World.ID))
	}
	script := NewScriptInfo(ln.World, id, code, ln.World.Context.Origin)

	// Determine source: URL or eval-parent?
	parentID, err := strconv.Atoi(src)
	if err != nil {
		// A string
		src, _ = StripQuotes(src)

		// URL-based script
		script.setURL(src)

		// Special case: is this a visible-v8:// script? (or a puppeteer-eval'd script?)
		if strings.HasSuffix(code, "//# sourceURL=__puppeteer_evaluation_script__\n)") || len(src) == 0 {
			script.VisibleV8 = true
		}
	} else {
		var parentScript *ScriptInfo
		parentScript, ok = ln.World.Scripts[parentID]
		if !ok {
			panic(fmt.Errorf("unknown parent script ID %d in isolate %s", parentID, ln.World.ID))
		}
		script.setEvaledBy(parentScript)
		script.VisibleV8 = parentScript.VisibleV8
	}

	ln.World.Scripts[id] = script

	return script
}

func (ln *LogInfo) changeScript(id int) {
	script, ok := ln.World.Scripts[id]
	if !ok {
		panic(fmt.Errorf("changing to undefined script ID %d in isolate %s", id, ln.World.ID))
	}
	ln.World.Context.Script = script
}

func (ln *LogInfo) changeOrigin(url string) {
	ln.World.Context.Origin = url
}

// NewIsolateInfo constructs a fresh, empty IsolateInfo for a given hex-string pointer tag
func NewIsolateInfo(id string) *IsolateInfo {
	return &IsolateInfo{
		ID:      id,
		Scripts: make(map[int]*ScriptInfo)}
}

func (iso *IsolateInfo) resetContext() {
	iso.Context.Script = nil
}

// ScriptHash was originally just an alias for SHA2-256 digest, but then we discovered collisions; now it's a tuple (length, SHA2-256, SHA3-256)
type ScriptHash struct {
	Length int
	SHA2   [sha256.Size]byte
	SHA3   [32]byte
}

// NewScriptHash produces a new (length, SHA2-256, SHA3-256) triple from a JS code string (i.e., a script)
func NewScriptHash(code string) ScriptHash {
	return ScriptHash{
		Length: len(code),
		SHA2:   sha256.Sum256([]byte(code)),
		SHA3:   sha3.Sum256([]byte(code)),
	}
}

// NewScriptInfo constructs a new script in a given Isolate with the given runtime ID and code body
func NewScriptInfo(iso *IsolateInfo, id int, code string, activeOrigin string) *ScriptInfo {
	return &ScriptInfo{
		Isolate:     iso,
		ID:          id,
		Code:        code,
		CodeHash:    NewScriptHash(code),
		FirstOrigin: activeOrigin,
	}
}

func (script *ScriptInfo) setURL(url string) {
	script.URL = url
}

func (script *ScriptInfo) setEvaledBy(parent *ScriptInfo) {
	script.EvaledBy = parent
}

// IngestStream is the entry point for parsing a given log and feeding the records into zero or more aggregators
func (ln *LogInfo) IngestStream(stream io.Reader, aggs ...Aggregator) error {
	// Read lines from input
	scan := bufio.NewScanner(stream)

	// Support LOOOONG lines
	scan.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), 128*1024*1024)

	// Start processing log lines
	var lineCount int
	var byteCount int64
	for scan.Scan() {
		line := scan.Bytes()
		lineCount++
		byteCount += int64(len(line)) + 1
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
				ln.addScript(scriptID, fields[1], fields[2])
			case '!':
				scriptID, err := strconv.Atoi(fields[0])
				if err != nil {
					ln.resetContext()
				} else {
					ln.changeScript(scriptID)
				}
			case '@':
				originString, _ := StripQuotes(fields[0])
				ln.changeOrigin(originString)
			default:
				for _, agg := range aggs {
					err := agg.IngestRecord(&ln.World.Context, lineCount, code, fields)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	if scan.Err() != nil {
		return scan.Err()
	}
	ln.Stats.Lines = lineCount
	ln.Stats.Bytes = byteCount
	log.Printf("%d lines (%d bytes) processed\n", ln.Stats.Lines, ln.Stats.Bytes)

	return nil
}
