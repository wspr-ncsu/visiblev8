package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.ncsu.edu/jjuecks/vv8-post-processor/causality"
	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
	"github.ncsu.edu/jjuecks/vv8-post-processor/elements"
	"github.ncsu.edu/jjuecks/vv8-post-processor/features"
	"github.ncsu.edu/jjuecks/vv8-post-processor/mega"
	"github.ncsu.edu/jjuecks/vv8-post-processor/micro"
)

// Version is set during build (to the git hash of the compiled code)
var Version string

// ---------------------------------------------------------------------------
// Aggregator selection/configuration logic
// ---------------------------------------------------------------------------

// formatAggregator ties together a distinct name (for de-duping) and constructor function for each supported aggregation pass
type formatAggregator struct {
	Tag  string
	Ctor func() (core.Aggregator, error)
}

// nullCtor provides no actual implementation of this--useful for no-op aggregation (dumping logs, importing logs, annotating, etc.)
func nullCtor() (core.Aggregator, error) {
	return nil, nil
}

// acceptedOutputFormats is the master map of supported aggregators and their short-names used by the CLI
var acceptedOutputFormats = map[string]formatAggregator{
	"Mfeatures":         {"MegaFeatureUsage", mega.NewAggregator},
	"features":          {"FeatureUsage", features.NewFeatureUsageAggregator},
	"poly_features":     {"FeatureUsage", features.NewFeatureUsageAggregator},
	"scripts":           {"FeatureUsage", features.NewFeatureUsageAggregator},
	"blobs":             {"FeatureUsage", features.NewFeatureUsageAggregator},
	"causality":         {"ScriptCausality", causality.NewScriptCausalityAggregator},
	"causality_graphml": {"ScriptCausality", causality.NewScriptCausalityAggregator},
	"create_element":    {"CreateElement", elements.NewCreateElementAggregator},
	"ufeatures":         {"MicroFeatureUsage", micro.NewFeatureUsageAggregator},
	"uscripts":          {"MicroScripts", micro.NewScriptsAggregator},
	"noop":              {"Noop", nullCtor},
}

// makeAggregators provides data-driven Aggregator instanciation
func makeAggregators(formats core.FormatSet) ([]core.Aggregator, error) {
	aggregators := make([]core.Aggregator, 0, len(formats))
	alreadyMade := make(core.FormatSet)

	for format := range formats {
		fa := acceptedOutputFormats[format]
		if !alreadyMade[fa.Tag] {
			alreadyMade[fa.Tag] = true
			agg, err := fa.Ctor()
			if err != nil {
				return nil, err
			} else if agg != nil {
				aggregators = append(aggregators, agg)
			}
		}
	}

	return aggregators, nil
}

// vv8LogNamePatter matches VV8 logfile name pattern (3 fields: name-stem, segment-rank, ".log")
var vv8LogNamePattern = regexp.MustCompile(`(vv8-[^.]+\.)(\d+)(\.log)$`)

// logSegment tracks order/name for a log stream segement file
type logSegment struct {
	rank int
	name string
}

// inputClusterMap groups/sorts raw input names (files and @oids) into sets of files (for on-disk logs) in a map
type inputClusterMap map[string][]logSegment

// getInputClusters handles grouping/sorting related log file segments on input
func getInputClusters(args []string) (inputClusterMap, error) {
	inputs := make(inputClusterMap)
	for _, val := range args {
		fields := vv8LogNamePattern.FindStringSubmatch(val)
		if len(fields) > 0 {
			key := fields[1] + "0.log"
			rank, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, err
			}
			inputs[key] = append(inputs[key], logSegment{rank, val})
		} else {
			// "@oid" (no input files) or "-" (stdin)
			inputs[val] = []logSegment{logSegment{0, val}}
		}
	}
	for _, files := range inputs {
		if len(files) > 0 {
			sort.Slice(files, func(i, j int) bool {
				return files[i].rank < files[j].rank
			})
		}
	}
	return inputs, nil
}

// ---------------------------------------------------------------------------
// Main entry point
// ---------------------------------------------------------------------------

type archiveStream struct {
	pageID    bson.ObjectId // externally provided PageID OID for this log's scope
	fileName  string        // globally unique filename (hostname:vv8logfilenamewithtimestamps)
	zipStream *gzip.Writer  // close this first
	mgoStream *mgo.GridFile // then close this
}

// Commit the archive stream (flushing/closing) and upsert a vv8log record for it
func (archive *archiveStream) Commit(db *mgo.Database, ln *core.LogInfo) error {
	err := archive.zipStream.Close()
	if err != nil {
		return err
	}
	err = archive.mgoStream.Close()
	if err != nil {
		return err
	}
	setDoc := bson.M{
		"last_update": bson.Now(),
		"origSize":    ln.Stats.Bytes,
		"pageId":      archive.pageID,
		"tagged":      (ln.Job != ""),
	}
	if (ln.Job != "") && (ln.Job != archive.pageID.Hex()) {
		log.Printf("warning: logfile '%s' contained job/pageID '%s', not '%s' (external pageID)",
			ln.RootName,
			ln.Job,
			archive.pageID.Hex())
		setDoc["taggedId"] = ln.Job // available only on tag mismatch, for post-mortems
	}
	change, err := db.C("vv8logs").Upsert(bson.M{
		"root_name": archive.fileName,
	}, bson.M{"$set": setDoc})
	if err != nil {
		return err
	}
	ln.ID = change.UpsertedId.(bson.ObjectId)
	log.Printf("successfully archived '%s' as %s", archive.fileName, change.UpsertedId.(bson.ObjectId).Hex())
	return nil
}

type kpwInvocation struct {
	Argv []string `json:"argv"`
}

// launchWebhookServer launches an HTTP listener bound to <listen> (host:port combo)
// handles POSTs (body content-type: JSON) to "/kpw/vv8-post-processor" (calling invoke)
// handles GETs to "/ready" (no body, always HTTP 200; used for readiness check/heartbeat)
func launchWebhookServer(listen string) error {
	http.HandleFunc("/kpw/vv8-post-processor", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "must POST a JSON array of arguments", http.StatusBadRequest)
			log.Printf("kpw-worker: non-POST received at dispatcher endpoint?!\n")
			return
		}
		var msg kpwInvocation
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, fmt.Sprintf("JSON decode error (%v)\n", err), http.StatusBadRequest)
			log.Printf("kpw-worker: JSON decode error (%v)\n", err)
			return
		}

		err := invoke(msg.Argv, false)
		if err != nil {
			http.Error(w, fmt.Sprintf("runtime error (%v)\n", err), http.StatusInternalServerError)
			log.Printf("kpw-worker: runtime error (%v)\n", err)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Nothing to do here but say "OK" (it tells them we're alive)
		log.Printf("kpw-worker: /ready OK (from %v)\n", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
	})
	log.Printf("webhook server listening at %s\n", listen)
	return http.ListenAndServe(listen, nil)
}

// invoke triggers the post-processor logic itself, controlled by the command-line arguments given in <args> (i.e., os.Args[1:])
// (if topLevel is TRUE, then invoke can launch a webhook-server triggers further invokes from HTTP POSTs)
func invoke(args []string, topLevel bool) error {
	var aggCtx core.AggregationContext
	var archivePageID string
	var aggPasses string
	var outputFormat string
	var dump, annotate, showVersion bool

	flags := flag.NewFlagSet("vv8PostProcessor", flag.ContinueOnError)
	var webhookListen string
	if topLevel {
		flags.StringVar(&webhookListen, "webhook-server", "", "launch kpw consumer HTTP server (POST to http://`LISTEN`/kpw/vv8-post-processor)")
	}
	flags.BoolVar(&showVersion, "version", false, "show version (Git commit hash) and quit")
	flags.BoolVar(&dump, "dump", false, "skip parsing/aggregating and just dump log to stdout")
	flags.BoolVar(&annotate, "annotate", false, "skip aggregating and dump JSON-annotated log lines to stdout (script/offset context, if any)")
	flags.BoolVar(&aggCtx.Archiving, "archive", false, "compress/archive logfile contents to Mongo during processing (REQUIRES -page-id)")
	flags.StringVar(&archivePageID, "page-id", "", "Mongo OID of `page` with which to associate log file/aggregates")
	flags.StringVar(&aggPasses, "aggs", "noop", "one or more ('+'-delimited) aggregation passes to perform")
	flags.StringVar(&outputFormat, "output", "stdout", "send data to `dest`; options: 'stdout', 'mongo', 'mongresql'")
	flags.StringVar(&aggCtx.RootName, "log-root", "", "manually specify root `name` for logfile")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s: [FLAGS] (-|FILENAME|@OID) [(-|FILENAME|@OID)...]\n", os.Args[0])
		flags.PrintDefaults()
		fmt.Fprintf(flags.Output(), "\nPasses Available:\n")
		for passName := range acceptedOutputFormats {
			fmt.Fprintf(flags.Output(), "\t%s\n", passName)
		}
	}
	flags.Parse(args)
	if showVersion {
		fmt.Println(Version)
		return nil
	}
	if webhookListen != "" {
		return launchWebhookServer(webhookListen)
	}
	if (flags.NArg() < 1) || // must have 1+ inputs
		(archivePageID != "" && !bson.IsObjectIdHex(archivePageID) || // -page-id's OID must be valid format (if given)
			(aggCtx.Archiving && archivePageID == "")) { // and `-archive` requires `-page-id OID`
		flags.Usage()
		return nil
	}

	// Parse the archive-mode option if non-empty
	if archivePageID != "" {
		aggCtx.ArchivePageID = bson.ObjectIdHex(archivePageID)
	}

	// Parse outputs (actualy, passes) from our sole positional argument
	if !(dump || annotate) {
		outputs := strings.Split(aggPasses, "+")
		aggCtx.Formats = make(core.FormatSet)
		for _, o := range outputs {
			if o == "dump" {
				// old-busted way of triggering a dump
				dump = true
				break
			} else {
				_, ok := acceptedOutputFormats[o]
				if ok {
					log.Printf("Output enabled: %s", o)
					aggCtx.Formats[o] = true
				} else {
					return fmt.Errorf("unknown output format '%s'", o)
				}
			}
		}
	}

	// Parse input names into grouped/sorted clusters
	inputClusters, err := getInputClusters(flags.Args())
	if err != nil {
		return fmt.Errorf("unable to parse input array %q", err)
	}

	// Process each cluster in turn
	for inputName, inputSegments := range inputClusters {
		// This is used only in the "archiving" special case (nil otherwise)
		var archive *archiveStream

		// Set up input stream (stdin if "-", Mongo vv8log OID if "@...", filename otherwise)
		var inputStream io.Reader
		if strings.HasPrefix(inputName, "@") {
			// Do we need to connect to Mongo, or was that already done?
			if aggCtx.MongoDb == nil {
				conn, err := core.DialMongo()
				if err != nil {
					log.Fatal(err)
				}
				log.Printf("Connected to Mongo @ %s\n", conn)
				defer conn.Session.Close() // tied to main()'s lifetime (why it's here and not in a helper)
				aggCtx.MongoDb = conn.Session.DB("")
			}

			// Parse OID
			oidHex := inputName[1:]
			if !bson.IsObjectIdHex(oidHex) {
				return fmt.Errorf("invalid oid '%s'", oidHex)
			}
			aggCtx.LogOid = bson.ObjectIdHex(oidHex)
			log.Printf("Reading vv8log OID %s\n", oidHex)

			// Get log record, check overrides, and open stream
			vv8log, err := core.GetVV8LogRecord(aggCtx.MongoDb, aggCtx.LogOid)
			if err != nil {
				log.Fatal(err)
			}
			inputStream, err = vv8log.Reader(aggCtx.MongoDb)
			if err != nil {
				log.Fatal(err)
			}

			// Set to match the Mongo data if it's missing
			aggCtx.RootName = vv8log.RootName
		} else if inputName == "-" {
			// Stdin (TODO support -archive, so long as the user provides a root-file name, too)
			inputStream = os.Stdin
			log.Println("Reading from stdin...")
		} else if len(inputSegments) > 0 {
			// Plain file input (assumed uncompressed, but possibly multi-segment)
			segmentStreams := make([]io.Reader, len(inputSegments))
			for i, segment := range inputSegments {
				log.Printf("Opening %s...\n", segment.name)
				file, err := os.Open(segment.name)
				if err != nil {
					log.Fatal(err)
				}
				segmentStreams[i] = core.NewClosingReader(file)
			}
			inputStream = io.MultiReader(segmentStreams...)

			// Special case--are we in "archiving" mode, and need to compress/stream the raw log into Mongo?
			// (Failures on the archival side become failures on the reader/aggregator side!)
			if aggCtx.Archiving {
				if aggCtx.MongoDb == nil {
					conn, err := core.DialMongo()
					if err != nil {
						log.Fatal(err)
					}
					log.Printf("Connected to Mongo @ %s\n", conn)
					defer conn.Session.Close() // tied to main()'s lifetime (why it's here and not in a helper)
					aggCtx.MongoDb = conn.Session.DB("")
				}
				hostName, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("fix your OS: %q", err)
				}
				archive = &archiveStream{
					pageID:   aggCtx.ArchivePageID,
					fileName: fmt.Sprintf("%s:%s", hostName, inputName),
				}
				archive.mgoStream, err = aggCtx.MongoDb.GridFS("fs").Create(archive.fileName)
				if err != nil {
					return err
				}
				archive.zipStream = gzip.NewWriter(archive.mgoStream)
				inputStream = io.TeeReader(inputStream, archive.zipStream)
				aggCtx.RootName = archive.fileName
			} else {
				aggCtx.RootName = inputName
			}
		} else {
			return fmt.Errorf("something is very wrong--where are the file names?")
		}

		// Handle output setup (with a special case for "dump" mode)
		var outputDriver core.DumpDriver
		if dump {
			count, err := io.Copy(os.Stdout, inputStream)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("%d bytes emitted\n", count)
			return nil
		} else if annotate {
			err := core.AnnotateStream(inputStream, &aggCtx)
			if err != nil {
				return err
			}
			return nil
		} else if outputFormat == "mongresql" || outputFormat == "mongo" {
			// Do we need to connect to Mongo, or was that already done?
			if aggCtx.MongoDb == nil {
				conn, err := core.DialMongo()
				if err != nil {
					return err
				}
				log.Printf("Connected to Mongo @ %s\n", conn)
				defer conn.Session.Close()
				aggCtx.MongoDb = conn.Session.DB("")
			}

			if outputFormat == "mongresql" {
				// Do we need to connect to PG? (we rely on the PGxxx environment variables being set...)
				if aggCtx.SQLDb == nil {
					aggCtx.SQLDb, err = sql.Open("postgres", "")
					if err != nil {
						return err
					}
					defer aggCtx.SQLDb.Close() // lifetime tied to main()
				}
				outputDriver = core.NewMongresqlDumpDriver(aggCtx.MongoDb, aggCtx.SQLDb)
			} else {
				outputDriver = core.NewMongoDumpDriver(aggCtx.MongoDb)
			}
		} else if outputFormat == "stdout" {
			outputDriver = core.NewStreamDumpDriver(os.Stdout)
		} else {
			return fmt.Errorf("unsupported output format '%s'", outputFormat)
		}

		// FINALLY build the aggregator array, post-processes that sucker, and feed the results into the output driver
		aggregators, err := makeAggregators(aggCtx.Formats)
		if err != nil {
			return err
		}

		aggCtx.Ln = core.NewLogInfo(aggCtx.LogOid, aggCtx.RootName)
		err = aggCtx.Ln.IngestStream(inputStream, aggregators...)
		if err != nil {
			return err
		}

		// If we were archiving, go ahead and flush/commit the archive streams now
		// (TODO: move this whole subsystem out into mongoz.go and simplify)
		if archive != nil {
			err = archive.Commit(aggCtx.MongoDb, aggCtx.Ln)
			if err != nil {
				return err
			}
		}

		for _, agg := range aggregators {
			err = outputDriver(agg, &aggCtx)
			if err != nil {
				return err
			}
		}

		// debugging hack--perform 1 second delay on no-op (null aggregator)
		if len(aggregators) == 0 {
			time.Sleep(time.Second)
		}
	}
	return nil
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	err := invoke(os.Args[1:], true)
	if err != nil {
		log.Fatal(err)
	}
}
