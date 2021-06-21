package mega

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
	"github.ncsu.edu/jjuecks/vv8-post-processor/features"
)

// Feature stores a distinct "raw Feature name", its components, and any IDL data found
type Feature struct {
	fullName     string       // The original name as recorded in the log (non-normalized)
	receiverName string       // The "object/interface" component of the original name
	memberName   string       // The "member" component of the original name
	idlInfo      core.IDLInfo // Any IDL base-interface/member-role data available
}

// Usage stores a distinct script-instance/execution-origin/feature-record/callsite/mode tuple
type Usage struct {
	script  *core.ScriptInfo // in what script instance
	origin  string           // under what SOP origin URL (execution context)
	feature *Feature         // was what feature used
	offset  int              // at what callsite (offset in bytes within script text)
	mode    rune             // in what way (g = get, s = set, c = call, n = new)
}

// usageAggregator tracks all scripts/instances/features/usages recorded in a VV8 logfile
type usageAggregator struct {
	idlTree     core.IDLTree        // IDL database
	features    map[string]*Feature // lookup map of distinct features seen
	usageCounts map[Usage]int       // counter of each distinct usage tuple we see
}

// NewAggregator creates a megaFeatures (Mfeatures) aggregator for scripts/instances/features/usages
func NewAggregator() (core.Aggregator, error) {
	idlTree, err := core.LoadDefaultIDLData()
	if err != nil {
		return nil, err
	}
	return &usageAggregator{
		idlTree:     idlTree,
		features:    make(map[string]*Feature),
		usageCounts: make(map[Usage]int),
	}, nil
}

// IngestRecord does nothing for MicroScriptsAggregators (since we care only about the scripts processed by the core framework)
func (agg *usageAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	// Only in a valid script/execution context...
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin != "") {
		// Raw field parsing/handling (offset, receiver/member, filtering, full-name)
		offset, err := strconv.Atoi(fields[0])
		if err != nil {
			return fmt.Errorf("%d: invalid script offset '%s'", lineNumber, fields[0])
		}
		var receiver, member string
		switch op {
		case 'g', 's':
			receiver, _ = core.StripCurlies(fields[1])
			member, _ = core.StripQuotes(fields[2])
		case 'n':
			receiver, _ = core.StripCurlies(fields[1])
			// Eliminate "native" prefix indicator from function names
			if strings.HasPrefix(receiver, "%") {
				receiver = receiver[1:]
			}
		case 'c':
			receiver, _ = core.StripCurlies(fields[2])
			member, _ = core.StripQuotes(fields[1])

			// Eliminate "native" prefix indicator from function names
			if strings.HasPrefix(member, "%") {
				member = member[1:]
			}
		default:
			return fmt.Errorf("%d: invalid mode '%c'; fields: %v", lineNumber, op, fields)
		}
		if features.FilterName(member) {
			// We have some names (V8 special cases, numeric indices) that are never useful
			return nil
		}
		var fullName string
		if member != "" {
			fullName = fmt.Sprintf("%s.%s", receiver, member)
		} else {
			fullName = receiver
		}

		// Feature-map lookup/population (with IDL lookup)
		feature, ok := agg.features[fullName]
		if !ok {
			feature = &Feature{
				fullName:     fullName,
				receiverName: receiver,
				memberName:   member,
			}
			agg.features[fullName] = feature
		}
		idlInfo, err := agg.idlTree.LookupInfo(receiver, member)
		if err == nil {
			feature.idlInfo = idlInfo
		}

		// Usage-map counting
		usage := Usage{
			script:  ctx.Script,
			origin:  ctx.Origin,
			feature: feature,
			offset:  offset,
			mode:    rune(op),
		}
		agg.usageCounts[usage]++
	}
	return nil
}

// DumpToStream sends feature/script/blob data to stdout for inspection
func (agg *usageAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	jstream := json.NewEncoder(stream)

	scriptHashID := 0
	scriptInstanceID := 0
	scriptHashes := make(map[core.ScriptHash]int)
	scriptInstances := make(map[*core.ScriptInfo]int)
	for _, iso := range ctx.Ln.Isolates {
		for _, script := range iso.Scripts {
			scriptHashID++
			scriptHashes[script.CodeHash] = scriptHashID
			scriptInstanceID++
			scriptInstances[script] = scriptInstanceID
		}
	}

	featureID := 0
	featureRecords := make(map[*Feature]int)
	for _, feature := range agg.features {
		featureID++
		featureRecords[feature] = featureID
	}

	for hash, id := range scriptHashes {
		jstream.Encode(core.JSONArray{"mega_script", core.JSONObject{
			"id":   id,
			"sha2": hex.EncodeToString(hash.SHA2[:]),
			"sha3": hex.EncodeToString(hash.SHA3[:]),
			"size": hash.Length,
		}})
	}

	for script, id := range scriptInstances {
		var evalParentID int
		if script.EvaledBy != nil {
			evalParentID = scriptInstances[script.EvaledBy]
		}

		jstream.Encode(core.JSONArray{"mega_instance", core.JSONObject{
			"id":             id,
			"script_id":      scriptHashes[script.CodeHash],
			"page_id":        ctx.Ln.PageID.Hex(),
			"logfile_id":     ctx.Ln.ID.Hex(),
			"isolate_ptr":    script.Isolate.ID,
			"runtime_id":     script.ID,
			"first_origin":   script.FirstOrigin,
			"load_url":       script.URL,
			"eval_parent_id": evalParentID,
		}})
	}

	for feature, id := range featureRecords {
		jstream.Encode(core.JSONArray{"mega_feature", core.JSONObject{
			"id":                id,
			"full_name":         feature.fullName,
			"receiver_name":     feature.receiverName,
			"member_name":       feature.memberName,
			"idl_base_receiver": feature.idlInfo.BaseInterface,
			"idl_member_role":   fmt.Sprintf("%c", feature.idlInfo.MemberRole),
		}})
	}

	usageID := 0
	for usage, count := range agg.usageCounts {
		usageID++
		jstream.Encode(core.JSONArray{"mega_usage", core.JSONObject{
			"id":          usageID,
			"instance_id": scriptInstances[usage.script],
			"feature_id":  featureRecords[usage.feature],
			"offset":      usage.offset,
			"mode":        fmt.Sprintf("%c", usage.mode),
			"count":       count,
		}})
	}

	return nil
}
