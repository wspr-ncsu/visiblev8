package core

// output driver interfaces/utilities

import (
	"database/sql"
	"io"
	"log"

	mgo "gopkg.in/mgo.v2"
)

// JSONArray is a friendly type alias for "array of any" (useful for constructing JSON arrays)
type JSONArray []interface{}

// JSONObject is a friendly type alias for "map of string to any" (useful for constructing JSON objects)
type JSONObject map[string]interface{}

// StreamDumper implements dumping of aggregate output data as some kind of plain text (typically for development/debugging)
type StreamDumper interface {
	DumpToStream(ctx *AggregationContext, stream io.Writer) error
}

// MongoDumper implements dumping of output data to Mongo backends only
type MongoDumper interface {
	DumpToMongo(ctx *AggregationContext, mongoDb *mgo.Database) error
}

// MongresqlDumper implements dumping of output data to Mongo and/or Postgres backends together
type MongresqlDumper interface {
	DumpToMongresql(ctx *AggregationContext, mongoDb *mgo.Database, sqlDb *sql.DB) error
}

// A DumpDriver is a function that invokes a given kind of output logic on the given aggregator (if possible)
type DumpDriver func(agg Aggregator, ctx *AggregationContext) error

// NewStreamDumpDriver creates a driver function to invoke StreamDumper logic on the given aggregator (if possible)
func NewStreamDumpDriver(stream io.Writer) DumpDriver {
	return func(agg Aggregator, ctx *AggregationContext) error {
		dumper, ok := agg.(StreamDumper)
		if ok {
			return dumper.DumpToStream(ctx, stream)
		}

		log.Printf("WARNING: %T does not support Stream dumping!", agg)
		return nil
	}
}

// NewMongoDumpDriver creates a driver function to invoke MongoDumper logic on the given aggregator (if possible)
func NewMongoDumpDriver(mongoDb *mgo.Database) DumpDriver {
	return func(agg Aggregator, ctx *AggregationContext) error {
		dumper, ok := agg.(MongoDumper)
		if ok {
			return dumper.DumpToMongo(ctx, mongoDb)
		}

		log.Printf("WARNING: %T does not support Mongo dumping!", agg)
		return nil
	}
}

// NewMongresqlDumpDriver creates a driver function to invoke MongresqlDumper logic on the given aggregator if possible,
// falling back to MongoDumper logic if necessary (and possible)
func NewMongresqlDumpDriver(mongoDb *mgo.Database, sqlDb *sql.DB) DumpDriver {
	return func(agg Aggregator, ctx *AggregationContext) error {
		dumper, ok := agg.(MongresqlDumper)
		if ok {
			return dumper.DumpToMongresql(ctx, mongoDb, sqlDb)
		}

		// Try, as fallback, a pure-mongo dumper cast (since we can support that, too)
		mongoOnlyDumper, ok := agg.(MongoDumper)
		if ok {
			return mongoOnlyDumper.DumpToMongo(ctx, mongoDb)
		}

		// No dice
		log.Printf("WARNING: %T does not support Mongresql/Mongo dumping!", agg)
		return nil
	}
}
