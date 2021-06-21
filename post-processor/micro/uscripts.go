package micro

// Minimal script-dumping-only aggregator (initially used to find script-sha2/256 collisions in the BlindMen dataset)

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"

	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// ScriptsAggregator is an empty type that exists to maintain the facade of the Aggregator interface for script-dumping
type ScriptsAggregator struct{}

// NewScriptsAggregator is a convention-preserving no-op
func NewScriptsAggregator() (core.Aggregator, error) {
	return &ScriptsAggregator{}, nil
}

// IngestRecord does nothing for MicroScriptsAggregators (since we care only about the scripts processed by the core framework)
func (agg *ScriptsAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	// Hahahahaha
	return nil
}

func (agg *ScriptsAggregator) dumpScriptTuples(ln *core.LogInfo) ([]*core.ScriptInfo, error) {
	scripts := make([]*core.ScriptInfo, 0, 100)

	// Eliminate duplicate hashes
	scriptMap := make(map[core.ScriptHash]*core.ScriptInfo)
	for _, iso := range ln.Isolates {
		for _, script := range iso.Scripts {
			if !script.VisibleV8 {
				scriptMap[script.CodeHash] = script
			}
		}
	}
	for _, script := range scriptMap {
		scripts = append(scripts, script)
	}

	return scripts, nil
}

// DumpToMongresql just thunks-through to DumpToMongo (since MicroScriptsAggregator does nothing with PostgreSQL)
func (agg *ScriptsAggregator) DumpToMongresql(ctx *core.AggregationContext, mongoDb *mgo.Database, sqlDb *sql.DB) error {
	return agg.DumpToMongo(ctx, mongoDb)
}

// DumpToMongo stores script metadata in a "uscripts" collection
func (agg *ScriptsAggregator) DumpToMongo(ctx *core.AggregationContext, mongoDb *mgo.Database) error {
	if ctx.Formats["uscripts"] {
		scripts, err := agg.dumpScriptTuples(ctx.Ln)
		if err != nil {
			return err
		}

		dest := mongoDb.C("uscripts").Bulk()
		for _, script := range scripts {
			doc := bson.M{
				"log": ctx.Ln.ID,
				"sid": script.ID,
				"len": script.CodeHash.Length,
				"sh2": script.CodeHash.SHA2[:],
				"sh3": script.CodeHash.SHA3[:],
			}
			if script.URL != "" {
				doc["url"] = script.URL
			}
			if script.EvaledBy != nil {
				doc["eid"] = script.EvaledBy.ID
			}
			dest.Insert(doc)
		}

		_, err = dest.Run()
		if err != nil {
			return err
		}
		log.Printf("uscripts.DumpToMongo: inserted %d records into Mongo\n", len(scripts))

		err = core.MarkVV8LogComplete(mongoDb, ctx.Ln.ID, "uscripts")
		if err != nil {
			return err
		}
	}

	return nil
}

// DumpToStream sends feature/script/blob data to stdout for inspection
func (agg *ScriptsAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)
	if ctx.Formats["uscripts"] {
		records, err := agg.dumpScriptTuples(ctx.Ln)
		if err != nil {
			return err
		}

		for _, r := range records {
			doc := core.JSONObject{
				"log": ctx.Ln.ID,
				"sid": r.ID,
				"len": r.CodeHash.Length,
				"sh2": r.CodeHash.SHA2[:],
				"sh3": r.CodeHash.SHA3[:],
			}
			if r.URL != "" {
				doc["url"] = r.URL
			}
			if r.EvaledBy != nil {
				doc["eid"] = r.EvaledBy.ID
			}
			jstream.Encode(doc)
		}
	}

	return nil
}
