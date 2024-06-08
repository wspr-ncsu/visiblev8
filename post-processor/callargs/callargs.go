package callargs

// ---------------------------------------------------------------------------
// aggregator for tracking DOM element types created by Document.createElement
// ---------------------------------------------------------------------------
import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/wspr-ncsu/visiblev8/post-processor/core"
)

type originCallsite struct {
	Origin  string           // what security origin
	Script  *core.ScriptInfo // What script
	Offset  int              // What location
	IDLName string           // API name of call
}

// CreateCallArgsAggregator tracks document.createElement calls and first-arguments (i.e., element type tags)
type CreateCallArgsAggregator struct {
	// IDL feature name normalization database
	idl core.IDLTree

	// track set of (lowercase) tag names Document.createElement()'d by a given originCallsite (origin/script/offset)
	callInfo map[originCallsite][][]string
}

// NewCreateElementAggregator constructs a CreateElementAggregator
func NewCreateCallArgsAggregator() (core.Aggregator, error) {
	tree, err := core.LoadDefaultIDLData()
	if err != nil {
		return nil, err
	}
	return &CreateCallArgsAggregator{
		idl:      tree,
		callInfo: make(map[originCallsite][][]string),
	}, nil
}

// IngestRecord parses a trace record, looking for document.createElement calls to track
func (agg *CreateCallArgsAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (op == 'c') && (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin.Origin != "") {
		offset, err := strconv.Atoi(fields[0])
		if err != nil {
			return fmt.Errorf("%d: invalid script offset '%s'", lineNumber, fields[0])
		}

		rcvr, _ := core.StripCurlies(fields[2])
		//TODO: split with comma and only grab the receiver
		name, _ := core.StripQuotes(fields[1])

		// Eliminate "native" prefix indicator from function names
		name = strings.TrimPrefix(name, "%")

		// We have some names (V8 special cases, numeric indices) that are never useful
		if core.FilterName(name) {
			return nil
		}

		// Normalize IDL names
		fullName, err := agg.idl.NormalizeMember(rcvr, name)
		if err != nil {
			fullName = fmt.Sprintf("%s.%s", rcvr, name)
		}

		// Create the call site to record
		cite := originCallsite{ctx.Origin.Origin, ctx.Script, offset, fullName}
		args := agg.callInfo[cite]
		if args == nil {
			args = make([][]string, 0)
			agg.callInfo[cite] = args
		}
		agg.callInfo[cite] = append(agg.callInfo[cite], fields[3:])
	}
	return nil
}

// DumpToStream implementation for create-elements
func (agg *CreateCallArgsAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)
	if ctx.Formats["callargs"] {
		for site, args := range agg.callInfo {
			jstream.Encode(core.JSONArray{"callargs", core.JSONObject{
				"script_hash":     hex.EncodeToString(site.Script.CodeHash.SHA2[:]),
				"script_offset":   site.Offset,
				"security_origin": site.Origin,
				"api_name":        site.IDLName,
				"passed_args":     core.JSONArray{args},
			}})
		}
	}
	return nil
}

//TODO: add BigQuery dump calls
//TODO: add Mongo dump calls
