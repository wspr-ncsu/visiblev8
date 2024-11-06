package flow

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/wspr-ncsu/visiblev8/post-processor/core"
)

type Script struct {
	APIs []string
	info *core.ScriptInfo
}

func NewScript(info *core.ScriptInfo) *Script {
	return &Script{
		APIs: make([]string, 0),
		info: info,
	}
}

type flowAggregator struct {
	scriptList map[int]*Script
	lastAction string
}

func NewAggregator() (core.Aggregator, error) {
	return &flowAggregator{
		scriptList: make(map[int]*Script),
	}, nil
}

func (agg *flowAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin.Origin != "") {
		offset, err := strconv.Atoi(fields[0])
		if err != nil {
			return fmt.Errorf("%d: invalid script offset '%s'", lineNumber, fields[0])
		}
		var receiver, member string
		switch op {
		case 'g', 's':
			receiver, _ = core.StripCurlies(fields[1])
			member, _ = core.StripQuotes(fields[2])
		case 'n':
			receiver, _ = core.StripCurlies(fields[1])
			receiver = strings.TrimPrefix(receiver, "%")
		case 'c':
			receiver, _ = core.StripCurlies(fields[2])
			member, _ = core.StripQuotes(fields[1])

			member = strings.TrimPrefix(member, "%")
		default:
			return fmt.Errorf("%d: invalid mode '%c'; fields: %v", lineNumber, op, fields)
		}

		if core.FilterName(member) {
			// We have some names (V8 special cases, numeric indices) that are never useful
			return nil
		}

		if strings.Contains(receiver, ",") {
			receiver = strings.Split(receiver, ",")[1]
		}

		var fullName string
		if member != "" {
			fullName = fmt.Sprintf("%s.%s", receiver, member)
		} else {
			fullName = receiver
		}

		script, ok := agg.scriptList[ctx.Script.ID]

		if !ok {
			script = NewScript(ctx.Script)
			agg.scriptList[ctx.Script.ID] = script
		}

		currentAction := fmt.Sprint(offset) + string(',') + fullName + string(',') + string(op)

		if agg.lastAction[:len(agg.lastAction) - 2] == currentAction[:len(currentAction) - 2] && op == 'c' {
			script.APIs = script.APIs[:len(script.APIs) - 1]
		}

		script.APIs = append(script.APIs, currentAction)
	}

	return nil
}

var scriptFlowFields = [...]string{
	"isolate",
	"submissionid",
	"instanceid",
	"visiblev8",
	"code",
	"url",
	"evaled_by",
	"apis",
	"first_origin",
}

func (agg *flowAggregator) DumpToPostgresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {

	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn("linked_flow", scriptFlowFields[:]...))
	if err != nil {
		txn.Rollback()
		return err
	}

	log.Printf("scriptFlow: %d scripts analysed", len(agg.scriptList))

	for _, script := range agg.scriptList {
		evaledBy := script.info.EvaledBy

		evaledById := -1
		if evaledBy != nil {
			evaledById = evaledBy.ID
		}

		_, err = stmt.Exec(
			script.info.Isolate.ID,
			script.info.VisibleV8,
			ctx.Ln.SubmissionID,
			script.info.CodeHash,
			script.info.Code,
			script.info.URL,
			evaledById,
			pq.Array(script.APIs),
			script.info.FirstOrigin.Origin)

		if err != nil {
			txn.Rollback()
			return err
		}

	}

	err = stmt.Close()
	if err != nil {
		txn.Rollback()
		return err
	}
	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (agg *flowAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)

	for _, script := range agg.scriptList {
		evaledBy := script.info.EvaledBy

		evaledById := -1
		if evaledBy != nil {
			evaledById = evaledBy.ID
		}

		jstream.Encode(core.JSONArray{"script_flow", core.JSONObject{
			"ID":          script.info.ID,
			"Isolate":     script.info.Isolate.ID,
			"IsVisibleV8": script.info.VisibleV8,
			"Code":        script.info.Code,
			"URL":         script.info.URL,
			"IsEvaledBy":  evaledById,
			"FirstOrigin": script.info.FirstOrigin,
			"APIs":        script.APIs,
		}})
	}

	return nil
}
