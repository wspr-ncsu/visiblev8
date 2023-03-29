package core

// output driver interfaces/utilities

import (
	"database/sql"
	"io"
	"log"
)

// JSONArray is a friendly type alias for "array of any" (useful for constructing JSON arrays)
type JSONArray []interface{}

// JSONObject is a friendly type alias for "map of string to any" (useful for constructing JSON objects)
type JSONObject map[string]interface{}

// StreamDumper implements dumping of aggregate output data as some kind of plain text (typically for development/debugging)
type StreamDumper interface {
	DumpToStream(ctx *AggregationContext, stream io.Writer) error
}

// PostgresqlDumper implements dumping of output data to Mongo and/or Postgres backends together
type PostgresqlDumper interface {
	DumpToPostgresql(ctx *AggregationContext, sqlDb *sql.DB) error
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

// NewPostgresqlDumpDriver creates a driver function to invoke PostgresqlDumper logic on the given aggregator if possible,
// falling back to MongoDumper logic if necessary (and possible)
func NewPostgresqlDumpDriver(sqlDb *sql.DB) DumpDriver {
	return func(agg Aggregator, ctx *AggregationContext) error {
		dumper, ok := agg.(PostgresqlDumper)
		if ok {
			return dumper.DumpToPostgresql(ctx, sqlDb)
		}

		// No dice
		log.Printf("WARNING: %T does not support Postgresql/Mongo dumping!", agg)
		return nil
	}
}
