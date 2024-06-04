package micro

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"strings"

	"github.com/wspr-ncsu/visiblev8/post-processor/core"
)

// FeatureUsageAggregator implements the Aggregator interface for collecting minimal script API usage data
type FeatureUsageAggregator struct {
	// IDL feature name normalization database
	idl core.IDLTree

	// Map of [origin] -> [featureName] -> bool (used?)
	usage map[string]map[string]bool
}

// NewFeatureUsageAggregator constructs a new MicroFeatureUsageAggregator
func NewFeatureUsageAggregator() (core.Aggregator, error) {
	tree, err := core.LoadDefaultIDLData()
	if err != nil {
		return nil, err
	}
	return &FeatureUsageAggregator{
		idl:   tree,
		usage: make(map[string]map[string]bool),
	}, nil
}

// IngestRecord extracts minimal script API usage stats from each callsite record
func (agg *FeatureUsageAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin.Origin != "") {
		var rcvr, name, fullName string
		switch op {
		case 'g':
			rcvr, _ = core.StripCurlies(fields[1])
			name, _ = core.StripQuotes(fields[2])
		case 'c':
			rcvr, _ = core.StripCurlies(fields[2])
			name, _ = core.StripQuotes(fields[1])
			// Eliminate "native" prefix indicator from function names
			name = strings.TrimPrefix(name, "%")
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
		if core.FilterName(name) {
			return nil
		}

		if strings.Contains(rcvr, ",") {
			rcvr = strings.Split(rcvr, ",")[1]
		}

		// Compensate for OOP-polymorphism by normalizing names to their base IDL interface
		fullName, err := agg.idl.NormalizeMember(rcvr, name)
		if err != nil {
			return err
		}

		// We log only IDL-normalized members
		// Stick it in our aggregation map
		originSet, ok := agg.usage[ctx.Origin.Origin]
		if !ok {
			originSet = make(map[string]bool)
			agg.usage[ctx.Origin.Origin] = originSet
		}
		originSet[fullName] = true
	}

	return nil
}

// DumpToStream implementation for micro-feature-usage
func (agg *FeatureUsageAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)
	if ctx.Formats["ufeatures"] /* for stream output, do not worry about whether or not we have a page-id */ {
		completeFeatureSet := make(map[string]bool)
		doc := core.JSONObject{
			"logFile":     ctx.RootName,
			"allFeatures": nil,
		}
		for _, featureSet := range agg.usage {
			for name := range featureSet {
				completeFeatureSet[name] = true
			}
		}
		completeArray := make(core.JSONArray, 0, len(completeFeatureSet))
		for feature := range completeFeatureSet {
			completeArray = append(completeArray, feature)
		}

		doc["allFeatures"] = completeArray
		jstream.Encode(doc)
	}
	return nil
}

func (agg *FeatureUsageAggregator) DumpToPostgresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {
	if ctx.Formats["ufeatures"] {

		logID, err := ctx.Ln.InsertLogfile(sqlDb)
		if err != nil {
			return err
		}

		txn, err := sqlDb.Begin()
		if err != nil {
			return err
		}

		completeFeatureSet := make(map[string]bool)
		for _, featureSet := range agg.usage {
			for name := range featureSet {
				completeFeatureSet[name] = true
			}
		}
		completeArray := make(core.JSONArray, 0, len(completeFeatureSet))
		for feature := range completeFeatureSet {
			completeArray = append(completeArray, feature)
		}

		featuresArray, err := json.Marshal(completeArray)
		// We are creating this array show we should not have any errors
		if err != nil {
			return err
		}

		_, err = txn.Exec("INSERT INTO js_api_features_summary (logfile_id, all_features) VALUES ($1, $2)", logID, featuresArray)

		if err != nil {
			txn.Rollback()
			return err
		}

		err = txn.Commit()
		if err != nil {
			return err
		}

	}
	return nil

}
