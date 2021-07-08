package features

// log-info Aggregator implementation for feature-usage and related tasks:
// * script-creation records ("scripts")
// * script-blob-storage ("blobs")
// * feature usage from monomorphic callsites ("features")
// * feature usage from polymorphic callsites ("poly_features")

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/lib/pq"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
)

// UsageInfo provides the key type for our map of feature-tuple aggregates
type UsageInfo struct {
	// Active security origin
	Origin string

	// Active script
	Script *core.ScriptInfo

	// Character offset into script (determines "call site")
	Offset int

	// Feature name (generally, RECEVER_CTOR_NAME.ITEM_NAME)
	Name string

	// Usage mode ('g' get, 'c' call, 's' set, 'n' new-style call)
	Usage rune
}

type callsite struct {
	Script *core.ScriptInfo // What script
	Offset int              // What location
}

// FeatureUsageAggregator implements the Aggregator interface for heavy-weight feature usage aggregates and script creation/harvesting
type FeatureUsageAggregator struct {
	// IDL feature name normalization database
	idl core.IDLTree

	// How many times have we seen (origin, script, offset, feature, "g|c|s|n") in this log file
	usage map[UsageInfo]int

	// Polymorphic call site detection map {callsite: {featureInvoked: bool}}
	morphisms map[callsite]map[string]bool
}

// NewFeatureUsageAggregator constructs a new FeatureUsageAggregator
func NewFeatureUsageAggregator() (core.Aggregator, error) {
	tree, err := core.LoadDefaultIDLData()
	if err != nil {
		return nil, err
	}
	return &FeatureUsageAggregator{
		idl:       tree,
		usage:     make(map[UsageInfo]int),
		morphisms: make(map[callsite]map[string]bool),
	}, nil
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

// IngestRecord parses a trace/callsite record and aggregates API feature usage
func (agg *FeatureUsageAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin != "") {
		offset, err := strconv.Atoi(fields[0])
		if err != nil {
			return fmt.Errorf("%d: invalid script offset '%s'", lineNumber, fields[0])
		}

		var rcvr, name, fullName string
		switch op {
		case 'g':
			rcvr, _ = core.StripCurlies(fields[1])
			name, _ = core.StripQuotes(fields[2])
		case 'c':
			rcvr, _ = core.StripCurlies(fields[2])
			name, _ = core.StripQuotes(fields[1])
			// Eliminate "native" prefix indicator from function names
			if strings.HasPrefix(name, "%") {
				name = name[1:]
			}
		case 's':
			rcvr, _ = core.StripCurlies(fields[1])
			name, _ = core.StripQuotes(fields[2])
		case 'n':
			// Radical experiment: ignore all "new" records!
			return nil
		default:
			// Oops--what was this?
			log.Printf("%d: wat? %c: %v", lineNumber, op, fields)
			return nil
		}

		// We have some names (V8 special cases, numeric indices) that are never useful
		if FilterName(name) {
			return nil
		}

		// Compensate for OOP-polymorphism by normalizing names to their base IDL interface (so we can detect callsite polymorphism)
		fullName, err = agg.idl.NormalizeMember(rcvr, name)
		if err != nil {
			// Fall back to just the name as-is if normalization fails
			fullName = fmt.Sprintf("%s.%s", rcvr, name)
		}

		// Stick it in our aggregation map (counting)
		agg.usage[UsageInfo{ctx.Origin, ctx.Script, offset, fullName, rune(op)}]++

		// And track callsite polymorphism (we break feature tuple sets into mono/poly morphic for different aggregation queries)
		morphKey := callsite{ctx.Script, offset}
		morphMap := agg.morphisms[morphKey]
		if morphMap == nil {
			morphMap = make(map[string]bool)
			agg.morphisms[morphKey] = morphMap
		}
		morphMap[name] = true
	}

	return nil
}

// StreamDumper implementation for feature-usage
/*func (agg *FeatureUsageAggregator) DumpToStream(ctx AggregationContext, stream io.Writer) error {
	return fmt.Errorf("DumpToStream not implemented for FeatureUsageAggregator")
}*/

func (agg *FeatureUsageAggregator) dumpBlobs(ln *core.LogInfo, mongoDb *mgo.Database, sqlDb *sql.DB) error {
	blobMap := make(map[core.ScriptHash]string)
	for _, iso := range ln.Isolates {
		for _, script := range iso.Scripts {
			if !script.VisibleV8 {
				blobMap[script.CodeHash] = script.Code
			}
		}
	}
	log.Printf("blob: %d unique scripts to archive", len(blobMap))
	for scriptHash, scriptCode := range blobMap {
		metaDoc := bson.M{"type": "script"}
		if ln.PageID.Valid() {
			metaDoc["pageId"] = ln.PageID
		} else if ln.Job != "" {
			metaDoc["job"] = ln.Job
		}
		_, oid, err := core.CompressBlob(mongoDb, "", []byte(scriptCode), metaDoc)
		if err != nil {
			return err
		}
		log.Printf("blob: script %s -> oid %s", hex.EncodeToString(scriptHash.SHA2[:]), oid)
	}
	if err := core.MarkVV8LogComplete(mongoDb, ln.ID, "blobs"); err != nil {
		return err
	}
	return nil
}

var featureUsageFields = [...]string{
	"logfile_id",
	"visit_domain",
	"security_origin",
	"script_hash",
	"script_offset",
	"feature_name",
	"feature_use",
	"use_count",
}

var scriptCreationFields = [...]string{
	"logfile_id",
	"visit_domain",
	"script_hash",
	"script_url",
	"eval_parent_hash",
	"isolate_ptr",
	"runtime_id",
	"first_origin",
}

// InsertLogfile inserts (if not present) a record about this log file into PG
func InsertLogfile(sqldb *sql.DB, ln *core.LogInfo) (int, error) {
	query := `INSERT INTO logfile
(mongo_id, job_id, root_name, size, lines) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING`
	var njobID sql.NullString
	if ln.Job != "" {
		njobID.Valid = true
		njobID.String = ln.Job
	}
	_, err := sqldb.Exec(query, ln.ID, njobID, ln.RootName, ln.Stats.Bytes, ln.Stats.Lines)
	if err != nil {
		return 0, err
	}

	var logID int
	err = sqldb.QueryRow(`SELECT id FROM logfile WHERE mongo_id = $1`, ln.ID).Scan(&logID)
	if err != nil {
		return 0, err
	}
	return logID, nil
}

type featureTupleRecord struct {
	securityOrigin string
	scriptHash     []byte
	scriptOffset   int
	featureName    string
	featureUse     rune
	useCount       int
}

type featureTupleSet struct {
	tuples     []featureTupleRecord
	suppressed int
}

func (agg *FeatureUsageAggregator) dumpFeatureTuples(ln *core.LogInfo) (featureTupleSet, error) {
	var result featureTupleSet

	workTuples := make([]featureTupleRecord, 0, 128)
	for key, count := range agg.usage {
		// Verify that this callsite (scriptHash/offset) has not been identified as polymorphic
		morph := agg.morphisms[callsite{key.Script, key.Offset}]
		if len(morph) < 2 {
			workTuples = append(workTuples, featureTupleRecord{
				securityOrigin: key.Origin,
				scriptHash:     key.Script.CodeHash.SHA2[:],
				scriptOffset:   key.Offset,
				featureName:    key.Name,
				featureUse:     key.Usage,
				useCount:       count,
			})
		} else {
			result.suppressed++
		}
	}
	result.tuples = workTuples

	return result, nil
}

func (agg *FeatureUsageAggregator) storeFeatureTuplesMongresql(ln *core.LogInfo, mongoDb *mgo.Database, sqlDb *sql.DB) error {
	results, err := agg.dumpFeatureTuples(ln)
	if err != nil {
		return err
	}

	// First, look up our Job's alexa domain
	visitDomain, err := core.GetRootDomain(mongoDb, ln)
	if err != nil {
		return err
	}

	// Insert record (if necessary) for logfile itself
	logID, err := InsertLogfile(sqlDb, ln)
	if err != nil {
		return err
	}

	// Main, bulk insert of tuples
	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}
	stmt, err := txn.Prepare(pq.CopyIn("feature_usage", featureUsageFields[:]...))
	if err != nil {
		txn.Rollback()
		return err
	}

	for _, tuple := range results.tuples {
		_, err = stmt.Exec(
			logID,
			visitDomain,
			tuple.securityOrigin,
			tuple.scriptHash,
			tuple.scriptOffset,
			tuple.featureName,
			string(tuple.featureUse),
			tuple.useCount)
		if err != nil {
			txn.Rollback()
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		txn.Rollback()
		return err
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

	// Update the Mongo document store about the completed analysis
	if err := core.MarkVV8LogComplete(mongoDb, ln.ID, "features"); err != nil {
		return err
	}

	// TODO: log this to Mongo as well, for reference
	log.Printf("features: emitted %d feature-usage tuples (%d suppressed for polymorphic callsite)",
		len(results.tuples),
		results.suppressed)
	return nil
}

func (agg *FeatureUsageAggregator) dumpPolyFeatureTuples(ln *core.LogInfo, mongoDb *mgo.Database, sqlDb *sql.DB) error {
	// First, look up our Job's alexa domain
	visitDomain, err := core.GetRootDomain(mongoDb, ln)
	if err != nil {
		return err
	}

	// Insert record (if necessary) for logfile itself
	logID, err := InsertLogfile(sqlDb, ln)
	if err != nil {
		return err
	}

	// Main, bulk insert of tuples
	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}
	stmt, err := txn.Prepare(pq.CopyIn("poly_feature_usage", featureUsageFields[:]...))
	if err != nil {
		txn.Rollback()
		return err
	}

	tupleCount := 0
	suppressCount := 0
	for key, count := range agg.usage {
		// Verify that this callsite (scriptHash/offset) has not been identified as monomorphic (i.e., it IS polymorphic)
		morph := agg.morphisms[callsite{key.Script, key.Offset}]
		if len(morph) >= 2 {
			// Insert usage record
			_, err = stmt.Exec(
				logID,
				visitDomain,
				key.Origin,
				key.Script.CodeHash.SHA2[:],
				key.Offset,
				key.Name,
				string(key.Usage),
				count)
			if err != nil {
				txn.Rollback()
				return err
			}
			tupleCount++
		} else {
			suppressCount++
			/*for feature, _ := range morph {
				log.Printf("features: polymorphic callsite (%s:%d): %s", hex.EncodeToString(key.Script.CodeHash[:]), key.Offset, feature)
			}*/
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		txn.Rollback()
		return err
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

	// Update the Mongo document store about the completed analysis
	if err := core.MarkVV8LogComplete(mongoDb, ln.ID, "poly_features"); err != nil {
		return err
	}
	// TODO: log this to mongo as well, for future reference
	log.Printf("poly_features: emitted %d feature-usage tuples (%d suppressed for monomorphic callsite)", tupleCount, suppressCount)
	return nil
}

func (agg *FeatureUsageAggregator) dumpScriptTuples(ln *core.LogInfo) ([]*core.ScriptInfo, error) {
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
	log.Printf("features.scripts: %d script creation records", len(scriptMap))
	for _, script := range scriptMap {
		scripts = append(scripts, script)
	}

	return scripts, nil
}

func (agg *FeatureUsageAggregator) storeScriptTuplesMongresql(ctx *core.AggregationContext, mongoDb *mgo.Database, sqlDb *sql.DB) error {
	records, err := agg.dumpScriptTuples(ctx.Ln)
	if err != nil {
		return err
	}

	// Look up our Job's alexa domain
	visitDomain, err := core.GetRootDomain(mongoDb, ctx.Ln)
	if err != nil {
		return err
	}

	// Insert record (if necessary) for logfile itself
	logID, err := InsertLogfile(sqlDb, ctx.Ln)
	if err != nil {
		return err
	}

	// Main, bulk insert of tuples
	txn, err := sqlDb.Begin()
	if err != nil {
		return err
	}
	stmt, err := txn.Prepare(pq.CopyIn("script_creation", scriptCreationFields[:]...))
	if err != nil {
		txn.Rollback()
		return err
	}

	// Loop over all script tuple-sources and emit a tuple for each (remember to make script_url and eval_parent_hash NULL if not present)
	for _, script := range records {
		var nullableURL interface{}
		var nullableParentHash interface{}
		if script.URL != "" {
			nullableURL = script.URL
		}
		if script.EvaledBy != nil {
			nullableParentHash = script.EvaledBy.CodeHash.SHA2[:]
		}
		_, err = stmt.Exec(logID, visitDomain, script.CodeHash.SHA2[:], nullableURL, nullableParentHash, script.Isolate.ID, script.ID, script.FirstOrigin)
		if err != nil {
			txn.Rollback()
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		txn.Rollback()
		return err
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

	// Update the Mongo document store about the completed analysis
	if err := core.MarkVV8LogComplete(mongoDb, ctx.Ln.ID, "scripts"); err != nil {
		return err
	}
	return nil
}

// DumpToMongresql handles tuple and blob insertion for feature usage (mono/polymorphic callsites), script creation, and script archiving
func (agg *FeatureUsageAggregator) DumpToMongresql(ctx *core.AggregationContext, mongoDb *mgo.Database, sqlDb *sql.DB) error {
	// Dump [monomorphic callsite] usage tuples into Postgres
	if ctx.Formats["features"] {
		err := agg.storeFeatureTuplesMongresql(ctx.Ln, mongoDb, sqlDb)
		if err != nil {
			return err
		}
	}

	// Dump [polymorphic callsite] usage tuples into Postgres
	if ctx.Formats["poly_features"] {
		err := agg.dumpPolyFeatureTuples(ctx.Ln, mongoDb, sqlDb)
		if err != nil {
			return err
		}
	}

	// Dump script tuples into Postgres
	if ctx.Formats["scripts"] {
		err := agg.storeScriptTuplesMongresql(ctx, mongoDb, sqlDb)
		if err != nil {
			return err
		}
	}

	// Dump script content blobs into Mongo
	if ctx.Formats["blobs"] {
		err := agg.dumpBlobs(ctx.Ln, mongoDb, sqlDb)
		if err != nil {
			return err
		}
	}

	return nil
}

// DumpToStream sends feature/script/blob data to stdout for inspection
func (agg *FeatureUsageAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)

	if ctx.Formats["features"] {
		result, err := agg.dumpFeatureTuples(ctx.Ln)
		if err != nil {
			return err
		}
		for _, r := range result.tuples {
			doc := core.JSONObject{
				"security_origin": r.securityOrigin,
				"script_hash":     hex.EncodeToString(r.scriptHash),
				"script_offset":   r.scriptOffset,
				"feature_name":    r.featureName,
				"feature_use":     string(r.featureUse),
				"use_count":       r.useCount,
			}
			jstream.Encode(core.JSONArray{"feature_usage", doc})
		}
	}

	if ctx.Formats["scripts"] {
		records, err := agg.dumpScriptTuples(ctx.Ln)
		if err != nil {
			return err
		}

		for _, r := range records {
			var eph interface{}
			if r.EvaledBy != nil {
				eph = hex.EncodeToString(r.EvaledBy.CodeHash.SHA2[:])
			}
			doc := core.JSONObject{
				"script_hash":      hex.EncodeToString(r.CodeHash.SHA2[:]),
				"script_url":       r.URL,
				"eval_parent_hash": eph,
				"isolate_ptr":      r.Isolate.ID,
				"runtime_id":       r.ID,
				"first_origin":     r.FirstOrigin,
			}
			jstream.Encode(core.JSONArray{"script_creation", doc})
		}
	}

	return nil
}
