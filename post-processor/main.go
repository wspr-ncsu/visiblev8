package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"

	"github.ncsu.edu/jjuecks/vv8-post-processor/callargs"
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
	"callargs":          {"CallArguments", callargs.NewCreateCallArgsAggregator},
	"Mfeatures":         {"MegaFeatureUsage", mega.NewAggregator},
	"features":          {"FeatureUsage", features.NewFeatureUsageAggregator},
	"poly_features":     {"FeatureUsage", features.NewFeatureUsageAggregator},
	"scripts":           {"FeatureUsage", features.NewFeatureUsageAggregator},
	"blobs":             {"FeatureUsage", features.NewFeatureUsageAggregator},
	"causality":         {"ScriptCausality", causality.NewScriptCausalityAggregator},
	"causality_graphml": {"ScriptCausality", causality.NewScriptCausalityAggregator},
	"create_element":    {"CreateElement", elements.NewCreateElementAggregator},
	"ufeatures":         {"MicroFeatureUsage", micro.NewFeatureUsageAggregator},
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
			inputs[val] = []logSegment{{0, val}}
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

// invoke triggers the post-processor logic itself, controlled by the command-line arguments given in <args> (i.e., os.Args[1:])
// (if topLevel is TRUE, then invoke can launch a webhook-server triggers further invokes from HTTP POSTs)
func invoke(args []string, topLevel bool) error {
	var aggCtx core.AggregationContext
	var aggPasses string
	var outputFormat string
	var SubmissionID string
	var annotate, showVersion bool

	flags := flag.NewFlagSet("vv8PostProcessor", flag.ContinueOnError)
	flags.BoolVar(&showVersion, "version", false, "show version (Git commit hash) and quit")
	flags.StringVar(&SubmissionID, "submissionid", "", "manually specify a submission id to associate with logfiles (used for getting the URL that is being visited)")
	flags.BoolVar(&annotate, "annotate", false, "skip aggregating and dump JSON-annotated log lines to stdout (script/offset context, if any)")
	flags.StringVar(&aggPasses, "aggs", "noop", "one or more ('+'-delimited) aggregation passes to perform")
	flags.StringVar(&outputFormat, "output", "stdout", "send data to `dest`; options: 'stdout', 'postgresql'")
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

	if flags.NArg() < 1 {
		flags.Usage()
		return nil
	}

	if SubmissionID != "" {
		aggCtx.SubmissionID = uuid.MustParse(SubmissionID)
	}

	// Parse outputs (actualy, passes) from our sole positional argument
	if !annotate {
		outputs := strings.Split(aggPasses, "+")
		aggCtx.Formats = make(core.FormatSet)
		for _, o := range outputs {
			_, ok := acceptedOutputFormats[o]
			if ok {
				log.Printf("Output enabled: %s", o)
				aggCtx.Formats[o] = true
			} else {
				return fmt.Errorf("unknown output format '%s'", o)
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
				aggCtx.MongoDb = conn.Client.Database("admin")
			}

			// Parse OID
			oidHex := inputName[1:]
			aggCtx.LogOid, err = primitive.ObjectIDFromHex(oidHex)
			if err != nil {
				return fmt.Errorf("invalid oid '%s'", oidHex)
			}
			log.Printf("Reading vv8log OID %s\n", oidHex)
			bucket, err := gridfs.NewBucket(aggCtx.MongoDb)
			if err != nil {
				return err
			}
			stream, err := bucket.OpenDownloadStream(aggCtx.LogOid)
			if err != nil {
				return err
			}

			// Set to match the Mongo data if it's missing
			aggCtx.RootName = stream.GetFile().Name
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
			fmt.Printf("%s", aggCtx.LogOid)
			aggCtx.RootName = inputName
		} else {
			return fmt.Errorf("something is very wrong--where are the file names?")
		}

		// Handle output setup (with a special case for "dump" mode)
		var outputDriver core.DumpDriver
		if annotate {
			err := core.AnnotateStream(inputStream, &aggCtx)
			if err != nil {
				return err
			}
			return nil
		} else if outputFormat == "postgresql" {
			// Do we need to connect to PG? (we rely on the PGxxx environment variables being set...)
			if aggCtx.SQLDb == nil {
				aggCtx.SQLDb, err = sql.Open("postgres", "sslmode=disable")
				if err != nil {
					return err
				}
				defer aggCtx.SQLDb.Close() // lifetime tied to main()
			}
			outputDriver = core.NewPostgresqlDumpDriver(aggCtx.SQLDb)
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

		aggCtx.Ln = core.NewLogInfo(aggCtx.LogOid, aggCtx.RootName, aggCtx.SubmissionID)
		err = aggCtx.Ln.IngestStream(inputStream, aggregators...)
		if err != nil {
			return err
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
