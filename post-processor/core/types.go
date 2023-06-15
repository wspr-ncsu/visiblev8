package core

import (
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Core error values
//-------------------------------

// ErrNotImplemented means this feature is not implemented yet
var ErrNotImplemented = errors.New("not implemented")

// Essential interfaces/structs
//------------------------------

// Aggregator is the abstract interface of "payload" implementations (i.e., what we are actually trying to aggregate)
type Aggregator interface {
	IngestRecord(ctx *ExecutionContext, lineNumber int, op byte, fields []string) error
}

// FormatSet tracks which output formats are enabled (by name, without duplicates)
type FormatSet map[string]bool

// AggregationContext provides configuration/options
type AggregationContext struct {
	LogOid       primitive.ObjectID // if present, used to tag db.vv8log[_id: LogId] complete
	LogID        uuid.UUID          // a unique ID assigned to each and every log
	SubmissionID uuid.UUID          // if present, used to provide the URL of the specific submission being processed
	RootName     string             // if present, used to provide root-filename of possibly-fragmented log file
	Formats      FormatSet          // what formats are we outputting?
	Ln           *LogInfo           // actual context structure
	MongoDb      *mongo.Database    // shared MongoDB connection (may be nil)
	SQLDb        *sql.DB            // shared PG connection (may be nil)
}

// A LogInfo tracks all essential context information for a VV8 log under processing
type LogInfo struct {
	// A unique ID assigned to the log
	ID uuid.UUID

	// Database id of the vv8log record being processed
	MongoID primitive.ObjectID

	// Root filename of this log stream
	RootName string

	// What is this log's discovered submission ID?
	SubmissionID uuid.UUID

	// What is the current isolate for this log?
	World *IsolateInfo

	// Any other isolates we know about
	Isolates map[string]*IsolateInfo

	// Has a entry for this log been added to the database?
	Tabled bool

	// Statistics on log size
	Stats struct {
		Lines int
		Bytes int64
	}
}

// ExecutionContext provides context to a trace record: the active script and the enforced SOP domain (if any)
type ExecutionContext struct {
	Script *ScriptInfo
	Origin *Origin
}

type Origin struct {
	Origin              string
	OriginSecurityToken string
}

// IsolateInfo tracks a V8 isolate (i.e., script namespace, for our purposes) during processing
type IsolateInfo struct {
	// The original identifying "pointer" string
	ID string

	// What are all the script IDs in this isolate mapped to?
	Scripts map[int]*ScriptInfo

	// What is our current context (active script and security origin) in this isolate?
	Context ExecutionContext
}

// ScriptInfo bundles all metadata/data available about a logged script
type ScriptInfo struct {
	// Parent pointer
	Isolate *IsolateInfo

	// Self ID (per parent isolate)
	ID int

	// Is this a "visible-v8://" script (which shouldn't be included in usage stats)?
	VisibleV8 bool

	// Script code
	Code string

	// CodeHash tries very hard to uniquely identify the script by its (length, SHA2-256, SHA3-256) tuple
	CodeHash ScriptHash

	// URL from which this script was loaded (if any--eval'd scripts will have nil for this)
	URL string

	// What script eval'd us? (if any--we might not be an eval'd script)
	EvaledBy *ScriptInfo

	// Active origin at moment of creation in the logs?
	FirstOrigin *Origin
}
