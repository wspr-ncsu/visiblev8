package fptp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/lib/pq"
	"github.com/wspr-ncsu/visiblev8/post-processor/core"
	"golang.org/x/net/publicsuffix"
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
	firstPartyProperty *EntityProperty
}

func NewFptpAggregator() (core.Aggregator, error) {
	emap, err := loadEMap()

	if err != nil {
		return nil, err
	}

	return &fptpAggregator{
		scriptList:         make(map[int]*Script),
		eMap:               emap,
		firstPartyProperty: nil,
	}, nil
}

func (agg *fptpAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin.Origin != "") {
		_, ok := agg.scriptList[ctx.Script.ID]

		if !ok {
			script := NewScript(ctx.Script)
			agg.scriptList[ctx.Script.ID] = script
		}

	}

	return nil
}

func (agg *fptpAggregator) accessEntityPropertyMap(origin string) (*EntityProperty, error) {
	etldplusone, err := publicsuffix.EffectiveTLDPlusOne(origin)
	if err != nil {
		return nil, err
	}
	entity, ok := agg.eMap.EntityPropertyMap[etldplusone]
	if !ok {
		return nil, fmt.Errorf("no entity property found for origin %s", origin)
	}

	return entity, nil
}

var firstPartyThirdPartyFields = [...]string{
	"sha2",
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
	log.Printf("Dumping fptp to Postgresql...")
	var rootDomain string
	rootDomain, err := core.GetRootDomain(sqlDb, ctx.Ln)

	if err != nil {
		return err
	}

	if agg.firstPartyProperty == nil {

		rootURL, err := url.Parse(rootDomain)

		if err != nil {
			return err
		}

		rootURLOrigin := rootURL.Hostname()

		agg.firstPartyProperty, err = agg.accessEntityPropertyMap(rootURLOrigin)
		if err != nil {
			agg.firstPartyProperty = &EntityProperty{
				DisplayName: rootURLOrigin,
				Tracking:    0.0,
			}
			agg.eMap.EntityPropertyMap[rootURLOrigin] = agg.firstPartyProperty
		}
	}

	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn("thirdpartyfirstparty", firstPartyThirdPartyFields[:]...))
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

		originURL, err := url.Parse(script.info.FirstOrigin.Origin)

		if err != nil {
			return err
		}

		scriptURLOrigin := scriptURL.Hostname()
		originURLOrigin := originURL.Hostname()

		scriptProperty, err := agg.accessEntityPropertyMap(scriptURLOrigin)
		if err != nil {
			scriptProperty = &EntityProperty{
				DisplayName: scriptURLOrigin,
				Tracking:    0.0,
			}
			agg.eMap.EntityPropertyMap[scriptURLOrigin] = scriptProperty
		}
		tracking := scriptProperty.Tracking
		originProperty, err := agg.accessEntityPropertyMap(originURLOrigin)
		if err != nil {
			originProperty = &EntityProperty{
				DisplayName: originURLOrigin,
				Tracking:    0.0,
			}
			agg.eMap.EntityPropertyMap[originURLOrigin] = originProperty
		}

		_, err = stmt.Exec(
			script.info.CodeHash.SHA2[:],
			rootDomain,
			script.info.URL,
			script.info.FirstOrigin.Origin,
			agg.firstPartyProperty.DisplayName,
			scriptProperty.DisplayName,
			originProperty.DisplayName,
			scriptProperty.DisplayName != originProperty.DisplayName,
			scriptProperty.DisplayName != agg.firstPartyProperty.DisplayName,
			tracking,
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

		originURL, err := url.Parse(script.info.FirstOrigin.Origin)

		if err != nil {
			return err
		}

		scriptURLOrigin := scriptURL.Hostname()
		originURLOrigin := originURL.Hostname()

		scriptProperty, err := agg.accessEntityPropertyMap(scriptURLOrigin)
		if err != nil {
			scriptProperty = &EntityProperty{
				DisplayName: scriptURLOrigin,
				Tracking:    0.0,
			}
			agg.eMap.EntityPropertyMap[scriptURLOrigin] = scriptProperty
		}
		originProperty, err := agg.accessEntityPropertyMap(originURLOrigin)
		if err != nil {
			originProperty = &EntityProperty{
				DisplayName: originURLOrigin,
				Tracking:    0.0,
			}
			agg.eMap.EntityPropertyMap[originURLOrigin] = originProperty
		}

		jstream.Encode(core.JSONArray{"firstpartythirdparty", core.JSONObject{
			"SHA2":           script.info.CodeHash.SHA2[:],
			"URL":            script.info.URL,
			"FirstOrigin":    script.info.FirstOrigin.Origin,
			"ScriptProperty": scriptProperty.DisplayName,
			"OriginProperty": originProperty.DisplayName,
			"ThirdParty":     scriptProperty.DisplayName != originProperty.DisplayName,
			"Tracking":       scriptProperty.Tracking,
		}})
	}

	return nil
}
