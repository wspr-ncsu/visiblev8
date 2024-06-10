package idl_apis

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/wspr-ncsu/visiblev8/post-processor/core"
)

type idlApisAggregator struct {
	APIs    map[string][]string
	idlTree core.IDLTree
}

func NewAggregator() (core.Aggregator, error) {
	idlTree, err := core.LoadDefaultIDLData()
	if err != nil {
		return nil, err
	}
	return &idlApisAggregator{
		idlTree: idlTree,
		APIs:    make(map[string][]string, 0),
	}, nil
}

func (agg *idlApisAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
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

		fullName, err := agg.idlTree.NormalizeMember(receiver, member)
		if err != nil {
			if member != "" {
				fullName = fmt.Sprintf("%s.%s", receiver, member)
			} else {
				fullName = receiver
			}
		}
		if !agg.idlTree.IsAPIInIDLFile(op, receiver, member) {
			scriptData, ok := agg.APIs[fullName]
			if !ok {
				scriptData = make([]string, 0)
			}
			URL := ``
			if ctx.Script.URL != "" {
				URL = ctx.Script.URL
			} else {
				URL = ctx.Script.EvaledBy.URL
			}
			Origin := ``
			if ctx.Script.FirstOrigin.Origin != "" {
				Origin = ctx.Script.FirstOrigin.Origin
			} else {
				Origin = ctx.Script.EvaledBy.FirstOrigin.Origin
			}
			scriptData = append(scriptData, fmt.Sprintf("%c %s %s %s", op, strconv.Itoa(offset), URL, Origin))
			agg.APIs[fullName] = scriptData
		}
	}

	return nil
}

var idlApisApiFields = [...]string{
	// "id",
	"api",
	"script_list",
	"log_file_id",
	"root_domain",
}

func (agg *idlApisAggregator) DumpToPostgresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {
	logID, err := ctx.Ln.InsertLogfile(sqlDb)
	if err != nil {
		return err
	}

	rootDomain, err := core.GetRootDomain(sqlDb, ctx.Ln)
	if err != nil {
		return err
	}

	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn("idlapis", idlApisApiFields[:]...))
	if err != nil {
		txn.Rollback()
		return err
	}

	log.Printf("idlApis registered APIs: %d scripts analysed", len(agg.APIs))

	for api, scriptList := range agg.APIs {

		_, err = stmt.Exec(
			api,
			pq.Array(scriptList),
			logID,
			rootDomain,
		)

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
