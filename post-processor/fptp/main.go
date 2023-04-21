package fptp

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/url"

	"github.com/lib/pq"
	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
)

type Script struct {
	info *core.ScriptInfo
}

func NewScript(info *core.ScriptInfo) *Script {
	return &Script{
		info: info,
	}
}

type fptpAggregator struct {
	scriptList         map[int]*Script
	eMap               *EMap
	firstPartyProperty string
}

func NewAggregator() (core.Aggregator, error) {
	emap, err := loadEMap()

	if err != nil {
		return nil, err
	}

	return &fptpAggregator{
		scriptList:         make(map[int]*Script),
		eMap:               emap,
		firstPartyProperty: "",
	}, nil
}

func (agg *fptpAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin != "") {
		_, ok := agg.scriptList[ctx.Script.ID]

		if !ok {
			script := NewScript(ctx.Script)
			agg.scriptList[ctx.Script.ID] = script
		}

	}

	return nil
}

var firstPartyThirdPartyFields = [...]string{
	"visiblev8",
	"code",
	"root_domain",
	"url",
	"first_origin",
	"property_of_root_domain",
	"property_of_first_origin",
	"property_of_script",
	"is_script_third_party_with_root_domain",
	"is_script_third_party_with_first_origin",
	"script_origin_tracking_value",
}

func (agg *fptpAggregator) DumpToPostgresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {
	var rootDomain string
	if agg.firstPartyProperty == "" {
		rootDomain, err := core.GetRootDomain(sqlDb, ctx.Ln)

		if err != nil {
			return err
		}

		agg.firstPartyProperty = agg.eMap.EntityPropertyMap[rootDomain].DisplayName
	}

	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn("script_flow", firstPartyThirdPartyFields[:]...))
	if err != nil {
		txn.Rollback()
		return err
	}

	log.Printf("firstPartyThirdParty: %d scripts analysed", len(agg.scriptList))

	for _, script := range agg.scriptList {
		scriptURL, err := url.Parse(script.info.URL)

		if err != nil {
			return err
		}

		originURL, err := url.Parse(script.info.FirstOrigin)

		if err != nil {
			return err
		}

		scriptURLOrigin := scriptURL.Hostname()
		originURLOrigin := originURL.Hostname()

		scriptProperty := agg.eMap.EntityPropertyMap[scriptURLOrigin].DisplayName
		originProperty := agg.eMap.EntityPropertyMap[originURLOrigin].DisplayName

		_, err = stmt.Exec(
			script.info.VisibleV8,
			script.info.Code,
			rootDomain,
			script.info.URL,
			script.info.FirstOrigin,
			agg.firstPartyProperty,
			scriptProperty,
			originProperty,
			scriptProperty == originProperty,
			scriptProperty == agg.firstPartyProperty,
			agg.eMap.EntityPropertyMap[scriptURLOrigin].Tracking,
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

func (agg *fptpAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)

	for _, script := range agg.scriptList {
		scriptURL, err := url.Parse(script.info.URL)

		if err != nil {
			return err
		}

		originURL, err := url.Parse(script.info.FirstOrigin)

		if err != nil {
			return err
		}

		scriptURLOrigin := scriptURL.Hostname()
		originURLOrigin := originURL.Hostname()

		scriptProperty := agg.eMap.EntityPropertyMap[scriptURLOrigin].DisplayName
		originProperty := agg.eMap.EntityPropertyMap[originURLOrigin].DisplayName

		jstream.Encode(core.JSONArray{"firstpartythirdparty", core.JSONObject{
			"IsVisibleV8":    script.info.VisibleV8,
			"Code":           script.info.Code,
			"URL":            script.info.URL,
			"FirstOrigin":    script.info.FirstOrigin,
			"ScriptProperty": scriptProperty,
			"OriginProperty": originProperty,
			"ThirdParty":     scriptProperty == originProperty,
			"Tracking":       agg.eMap.EntityPropertyMap[scriptURLOrigin].Tracking,
		}})
	}

	return nil
}
