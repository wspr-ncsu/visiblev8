package micro

import (
	"encoding/json"
	"io"
	"log"
	"strings"

	mgo "gopkg.in/mgo.v2"

	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
	"github.ncsu.edu/jjuecks/vv8-post-processor/features"
)

// UsageInfo provides the key type for our map of feature-tuple aggregates
type UsageInfo struct {
	// Active security origin
	Origin string

	// Feature name (generally, RECEVER_CTOR_NAME.ITEM_NAME)
	Name string
}

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
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin != "") {
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
		if features.FilterName(name) {
			return nil
		}

		// Compensate for OOP-polymorphism by normalizing names to their base IDL interface
		fullName, err := agg.idl.NormalizeMember(rcvr, name)
		if err == nil {
			// We log only IDL-normalized members
			// Stick it in our aggregation map
			originSet, ok := agg.usage[ctx.Origin]
			if !ok {
				originSet = make(map[string]bool)
				agg.usage[ctx.Origin] = originSet
			}
			originSet[fullName] = true
		}
	}

	return nil
}

// DumpToStream implementation for micro-feature-usage
func (agg *FeatureUsageAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)
	if ctx.Formats["ufeatures"] /* for stream output, do not worry about whether or not we have a page-id */ {
		completeFeatureSet := make(map[string]bool)
		doc := core.JSONObject{
			"logFile":        ctx.RootName,
			"pageId":         ctx.Ln.PageID,
			"allFeatures":    nil,
			"featureOrigins": make(core.JSONArray, 0, len(agg.usage)),
		}
		for origin, featureSet := range agg.usage {
			featureSetArray := make(core.JSONArray, 0, len(featureSet))
			for name := range featureSet {
				featureSetArray = append(featureSetArray, name)
				completeFeatureSet[name] = true
			}
			doc["featureOrigins"] = append(doc["featureOrigins"].(core.JSONArray), core.JSONObject{
				"origin":   origin,
				"features": featureSetArray,
			})
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

// DumpToMongo implementation for micro-feature-usage
func (agg *FeatureUsageAggregator) DumpToMongo(ctx *core.AggregationContext, mongoDb *mgo.Database) error {
	if ctx.Formats["ufeatures"] && ctx.Ln.PageID.Valid() {
		completeFeatureSet := make(map[string]bool)
		doc := core.JSONObject{
			"logId":          ctx.Ln.ID,
			"pageId":         ctx.Ln.PageID,
			"allFeatures":    nil,
			"featureOrigins": make(core.JSONArray, 0, len(agg.usage)),
		}
		for origin, featureSet := range agg.usage {
			featureSetArray := make(core.JSONArray, 0, len(featureSet))
			for name := range featureSet {
				featureSetArray = append(featureSetArray, name)
				completeFeatureSet[name] = true
			}
			doc["featureOrigins"] = append(doc["featureOrigins"].(core.JSONArray), core.JSONObject{
				"origin":   origin,
				"features": featureSetArray,
			})
		}
		completeArray := make(core.JSONArray, 0, len(completeFeatureSet))
		for feature := range completeFeatureSet {
			completeArray = append(completeArray, feature)
		}
		doc["allFeatures"] = completeArray

		col := mongoDb.C("js_api_features")
		err := col.Insert(doc)
		if err != nil {
			return err
		}

		// Update the Mongo document store about the completed analysis
		if err := core.MarkVV8LogComplete(mongoDb, ctx.Ln.ID, "ufeatures"); err != nil {
			return err
		}

		log.Printf("ufeatures -- saved %d distinct features used by %d distinct origins to Mongo js_api_features", len(completeArray), len(agg.usage))
	}
	return nil
}
