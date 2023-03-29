# VV8 Post Processor

> **Note** This is fork of the post-processor repo located at the [visiblev8](https://github.com/wspr-ncsu/visiblev8)

Swiss-Army dumping ground of logic related to VisibleV8 (VV8) trace log processing.
Originally tightly integrated to a single workflow (i.e., many assumptions/dependencies w.r.t. databases and filenames); slowly being transmogrified into a standalone, modular toolkit.

## Building

Use a modern [Go](https://golang.org/) toolchain (e.g., 1.13 or newer).  Get the code (`git clone` or unZIP).  Run `go build` inside the project root.  The resulting `vv8-post-processor` binary is all you need (though some of the runtime modes require `idldata.json` as well).

## Quick Start

Assuming you have some VV8 log (e.g., `vv8-*.[0-9].log`) files in `$PWD`, you can quickly experiment with the supported modes/tools by letting the output go to `stdout` only (the default) and specifying a single "aggregator" (i.e., output mode) at a time.  E.g., to get a quick summary of features used by execution-context-origin-URL, you can do (note that this is an operation that relies on `idldata.json` being in `$PWD`, too):

```$ ./vv8-post-processor -aggs ufeatures vv8*.log```

The output is a single JSON object containing all browser API features accessed globally (an array of strings under the `allFeatures` key) and an array of per-distinct-origin-URL feature arrays (the `featureOrigins` key).

You can combined multiple aggregation passes in a single run by specifying a `+` delimited list of aggregator names as the argument to the `-aggs` flag when you run the post-processor.  (This approach typically makes sense more in a batch processing situation where outputs are being sent to databases.)

## Input

Log file input can be read from named log files or from `stdin` (by specifying `-` as a filename).
Filenames prefixed by the `@` character are interpreted as MongoDB OIDs from our original MongoDB storage scheme; these require MongoDB credentials to be provided via environment variables;

## Output Modes

By default, output goes to `stdout` (typically in some form of JSON, though each aggregator is free to use a different format).

The original workflow for which `vv8-post-processor` was written involved both MongoDB and PostgreSQL databases used in concert (for live collection of bulk data and for offline aggregation and analysis, respectively).  Hence, most aggregators support `mongo` (MongoDB I/O required) and/or `mongresql` (both MongoDB and PostgreSQL I/O required).  We do not document the particulars here, as we consider these modes to be deprecated for future development.  The source code (including a SQL DDL schema file for PostgreSQL) can provide details for the stubbornly intrepid.

That said, a subsequent PostgreSQL-based workflow (via the `Mfeatures` aggregator; see the `mega` folder for schema details) has proved useful and fairly scalable, so you might want to check that out.

## Other options

* `-submission-id`: Specify the submission ID to which the logs are linked to
* `-log-root`: a way to manually specify a base name for a log file when streaming data from `stdin`

## What are all these aggregators?

* `call_args` **(broken)**: A aggregator that records every call being made and the associated arguments
* `poly_features/features`/`scripts`/`blobs`: 4 different output modes for a single input-processing pass (the original one, actually) that extracts polymorphic and monomorphic feature sites (locations within scripts that used a given feature and how many times; polymorphic and monomorphic instances kept separate), loaded script hashes and metadata (i.e.,  URL or eval-parent hash), and the full binary dump of loaded scripts
* `create_element`**(broken)**: emits records of each call to `Document.createElement`, its script context/location, and its first argument (i.e., what kind of element was being created)
* `causality`/`causality_graphml` **(broken)**: 2 different output modes for a single input-processing pass that uses a bunch of heuristics to try to reconstruct script provenance (what script loaded what other script); the later mode emits GraphML (i.e., XML)
* `ufeatures`: a nice summary of features-touched globally on a per logfile basis
* `Mfeatures`: the latest and probably best/richest aggregation of data into a fairly normalized entity-relationship schema of script/instance/feature/usage; requires PostgreSQL (see `mega/postgres_schema.sql`)
