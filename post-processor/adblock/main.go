package adblock

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
)

type RawScript struct {
	info    *core.ScriptInfo
	blocked bool
}

type ScriptURLPair struct {
	URL     string `json:url`
	Origin  string `json:origin`
	Blocked bool   `json:blocked`
}

func NewScript(info *core.ScriptInfo) *RawScript {
	return &RawScript{
		info:    info,
		blocked: false,
	}
}

func NewScriptURLPair(ln []byte) (*ScriptURLPair, error) {
	var tmp ScriptURLPair
	err := json.Unmarshal(ln, &tmp)
	return &tmp, err
}

type adblockAggregator struct {
	scriptList         map[int]*RawScript
	urlPairList        []*ScriptURLPair
	adblockTmpFilename string
}

func NewAdblockAggregator() (core.Aggregator, error) {
	uu, err := uuid.NewUUID()

	if err != nil {
		log.Printf("unable to generate file name")
		return nil, err
	}

	var adblockTmpFilename = `/tmp/adblock-file-` + uu.String()
	return &adblockAggregator{
		urlPairList:        make([]*ScriptURLPair, 0),
		scriptList:         make(map[int]*RawScript),
		adblockTmpFilename: adblockTmpFilename,
	}, nil
}

func (agg *adblockAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin != "") {
		_, ok := agg.scriptList[ctx.Script.ID]

		if !ok {
			agg.scriptList[ctx.Script.ID] = NewScript(ctx.Script)
		}
	}

	return nil
}

func (agg *adblockAggregator) sendURLsToAdblock() error {
	adblockBinary := os.Getenv("ADBLOCK_BINARY")
	if adblockBinary == "" {
		adblockBinary = "./adblock"
	}

	var file, err_file = os.OpenFile(agg.adblockTmpFilename, os.O_RDWR|os.O_CREATE, 0644)

	if err_file != nil {
		return err_file
	}

	jstreamAdblock := json.NewEncoder(file)

	var cnt int = 0

	for _, script := range agg.scriptList {
		if script.info.URL == "" || script.info.FirstOrigin == "null" {
			continue
		}

		jstreamAdblock.Encode(core.JSONObject{
			"url":    script.info.URL,
			"origin": script.info.FirstOrigin,
		})
		cnt++
	}
	log.Printf("Sent %d scripts to adblock", cnt)
	file.Close()

	adblockProc := exec.Command(adblockBinary, agg.adblockTmpFilename)
	stdout, err := adblockProc.StdoutPipe()

	if err != nil {
		return err
	}

	log.Printf("Starting adblock process: %s", adblockBinary)

	adblockProc.Stderr = os.Stderr

	defer adblockProc.Wait()
	err = adblockProc.Start()

	if err != nil {
		return err
	}

	sc := bufio.NewScanner(stdout)
	maxCapacity := 1024 * 1024
	buf := make([]byte, maxCapacity)
	sc.Buffer(buf, maxCapacity)

	go agg.readFromAdblock(sc, cnt)

	return nil
}

func (agg *adblockAggregator) readFromAdblock(sc *bufio.Scanner, cnt int) error {
	for cnt > 0 && sc.Scan() {
		line := sc.Text()
		scriptURLPair, err := NewScriptURLPair([]byte(line))

		if err != nil {
			return err
		}

		agg.urlPairList = append(agg.urlPairList, scriptURLPair)
		cnt--
	}

	return nil
}

func (agg *adblockAggregator) DumpToPostgresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {
	log.Printf("Dumping %d scripts to adblock", len(agg.scriptList))
	err := agg.sendURLsToAdblock()

	log.Printf("%d number of scripts processed", len(agg.urlPairList))

	if err != nil {
		return err
	}

	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(`INSERT INTO adblock (url, origin, blocked) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`)
	if err != nil {
		txn.Rollback()
		return err
	}

	for _, script := range agg.urlPairList {

		_, err = stmt.Exec(
			script.URL,
			script.Origin,
			script.Blocked)

		if err != nil {
			txn.Rollback()
			return err
		}

	}

	log.Printf("%d number of scripts dumped to postgresql", len(agg.urlPairList))

	err = stmt.Close()
	if err != nil {
		txn.Rollback()
		return err
	}
	err = txn.Commit()
	if err != nil {
		return err
	}

	os.Remove(agg.adblockTmpFilename)

	return nil
}

func (agg *adblockAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream_output := json.NewEncoder(stream)
	err := agg.sendURLsToAdblock()

	if err != nil {
		return err
	}

	for _, script := range agg.urlPairList {
		if script.Blocked {
			jstream_output.Encode(core.JSONArray{"adblock", core.JSONObject{
				"FirstOrigin": script.Origin,
				"URL":         script.URL,
				"Blocked":     script.Blocked,
			}})
		}
	}

	os.Remove(agg.adblockTmpFilename)

	return nil
}
