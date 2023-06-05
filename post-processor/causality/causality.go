package causality

// ---------------------------------------------------------------------------
// aggregator for sniffing dynamic script inclusions/insertions
// ---------------------------------------------------------------------------

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"reflect"
	"strings"

	"github.com/lib/pq"
	"github.com/yaricom/goGraphML/graphml"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.ncsu.edu/jjuecks/vv8-post-processor/core"
)

type genesisLink struct {
	parent *core.ScriptInfo
	write  bool
}

// A ScriptCausalityAggregator reconstructs a tree of script causality from evals, inclusions, insertions, and document.writes...
type ScriptCausalityAggregator struct {
	// Map of URLs included in different execution contexts (well, scripts--we ought to retrofit full frame context once we have it)
	includeMap map[string]map[genesisLink]bool

	// Map of script code (by hash) dynamically inserted into the DOM in different execution contexts
	insertMap map[core.ScriptHash]map[genesisLink]bool

	// Map of execution context (origin/script) to document-write streams
	writeMap map[core.ExecutionContext]string
}

// NewScriptCausalityAggregator constructs a new ScriptCausalityAggregator
func NewScriptCausalityAggregator() (core.Aggregator, error) {
	return &ScriptCausalityAggregator{
		includeMap: make(map[string]map[genesisLink]bool),
		insertMap:  make(map[core.ScriptHash]map[genesisLink]bool),
		writeMap:   make(map[core.ExecutionContext]string),
	}, nil
}

// addInclusion records a script "inclusion," where a <script> DOM node has its src="..." attribute updated to a new URL
// (strip out scheme if the URL is parsable, for more forgiving URL matching)
func (agg *ScriptCausalityAggregator) addInclusion(rawURL string, actor *core.ExecutionContext, write bool) {
	cookedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("error parsing URL '%s' (%s); can't strip scheme out\n", rawURL, err)
	} else {
		cookedURL.Scheme = ""
		rawURL = cookedURL.String()
	}
	urlKey := fmt.Sprintf("%s$%s", actor.Origin, rawURL)

	exeSet := agg.includeMap[urlKey]
	if exeSet == nil {
		exeSet = make(map[genesisLink]bool)
		agg.includeMap[urlKey] = exeSet
	}
	exeSet[genesisLink{actor.Script, write}] = true
}

// addInsertion records a script "insertion," where a src-less <script> node has its text="..." attribute updated to a new script body
func (agg *ScriptCausalityAggregator) addInsertion(codeHash core.ScriptHash, actor *core.ExecutionContext, write bool) {
	exeSet := agg.insertMap[codeHash]
	if exeSet == nil {
		exeSet = make(map[genesisLink]bool)
		agg.insertMap[codeHash] = exeSet
	}
	exeSet[genesisLink{actor.Script, write}] = true
}

// IngestRecord processes a single trace record (line), looking for dynamic script causality
func (agg *ScriptCausalityAggregator) IngestRecord(ctx *core.ExecutionContext, lineNumber int, op byte, fields []string) error {
	if (ctx.Script != nil) && !ctx.Script.VisibleV8 && (ctx.Origin != "") {
		var name, rcvr string
		switch op {
		case 's':
			rcvr, _ = core.StripCurlies(fields[1])
			name, _ = core.StripQuotes(fields[2])
		case 'c':
			rcvr, _ = core.StripCurlies(fields[2])
			name, _ = core.StripQuotes(fields[1])
			// Eliminate "native" prefix indicator from function names
			if strings.HasPrefix(name, "%") {
				name = name[1:]
			}
		default:
			// Short-circuit: we don't handle anything else
			return nil
		}
		if (op == 's') && (rcvr == "HTMLScriptElement") {
			if name == "src" {
				// Remote inclusion...
				incURL, ok := core.StripQuotes(fields[3])
				if ok {
					agg.addInclusion(incURL, ctx, false)
				} else {
					log.Printf("%d: bogus HtmlScriptElement.src = %s\n", lineNumber, fields[3])
				}
			} else if name == "text" {
				// Inline insertion (TODO: add other comparable attributes)
				incSrc, ok := core.StripQuotes(fields[3])
				if ok {
					agg.addInsertion(core.NewScriptHash(incSrc), ctx, false)
				} else {
					log.Printf("%d: bogus HtmlScriptElement.text = %s\n", lineNumber, fields[3])
				}
			}
		} else if (op == 'c') && (rcvr == "HTMLDocument") && (name == "write") {
			// document.write(...) shenanigans!
			html, ok := "", false
			if len(fields) > 3 {
				html, ok = core.StripQuotes(fields[3])
			}
			if ok {
				agg.writeMap[*ctx] += html
			} else {
				log.Printf("%d, document.write(%s)...wat??", lineNumber, html)
			}

		}
	}
	return nil
}

type causalityRecord struct {
	child   *core.ScriptInfo
	parent  *core.ScriptInfo
	genesis string
	url     string
	pCard   int
	cCard   int
}

// Common dumping logic shared by all back-ends (returns a slice of records or an error)
func (agg *ScriptCausalityAggregator) causalityDumper(ctx *core.AggregationContext) ([]causalityRecord, error) {
	records := make([]causalityRecord, 0, 100)
	srcMap := make(map[string][]*core.ScriptInfo)
	codeMap := make(map[core.ScriptHash][]*core.ScriptInfo)

	// Remove a specific script instance from the codeMap
	removeCodeMapEntry := func(script *core.ScriptInfo) error {
		instances, ok := codeMap[script.CodeHash]
		if ok {
			for i, s := range instances {
				if s == script {
					a := instances
					copy(a[i:], a[i+1:])
					a[len(a)-1] = nil
					a = a[:len(a)-1]
					if len(a) > 0 {
						codeMap[script.CodeHash] = a
					} else {
						delete(codeMap, script.CodeHash)
					}
					break
				}
			}
			return nil
		}
		return fmt.Errorf("no such script (%s) found in codeMap", hex.EncodeToString(script.CodeHash.SHA2[:]))
	}

	// Build comprehensive maps of (url -> [scripts loaded from] and codeHash -> [scripts])
	// (deliberately strip out URL scheme to enable matching on scheme-relative URLs; this is a heuristic, remember?)
	// In the same pass, find and emit 'eval' causality links
	for _, iso := range ctx.Ln.Isolates {
		for _, script := range iso.Scripts {
			if !script.VisibleV8 {
				if script.EvaledBy != nil {
					records = append(records, causalityRecord{
						child:   script,
						genesis: "eval",
						parent:  script.EvaledBy,
					})
				} else {
					codeMap[script.CodeHash] = append(codeMap[script.CodeHash], script)
					if script.URL != "" {
						var urlKey string

						cookedURL, err := url.Parse(script.URL)
						if err != nil {
							log.Printf("error parsing URL '%s' (%s); can't strip scheme out\n", script.URL, err)
							urlKey = fmt.Sprintf("%s$%s", script.FirstOrigin, script.URL)
						} else {
							cookedURL.Scheme = ""
							urlKey = fmt.Sprintf("%s$%s", script.FirstOrigin, cookedURL.String())
						}

						srcMap[urlKey] = append(srcMap[urlKey], script)
					}
				}
			}
		}
	}

	// Parse document.write streams; find write-inserts and write-includes and add to the appropriate lists BEFORE emission
	for exeCtx, stream := range agg.writeMap {
		log.Printf("parsing %d-byte stream from (%s/%d)", len(stream), exeCtx.Origin, exeCtx.Script.ID)

		var script *html.Token
		var chunks string
		z := html.NewTokenizer(strings.NewReader(stream))

		for {
			tt := z.Next()
			if tt == html.ErrorToken {
				err := z.Err()
				if err == io.EOF {
					break
				} else {
					return nil, err
				}
			}

			tok := z.Token()
			if tok.Type == html.StartTagToken && tok.DataAtom == atom.Script {
				script = &tok
				for _, attr := range tok.Attr {
					if attr.Key == "src" {
						log.Printf("\tinclude '%s'", attr.Val)
						agg.addInclusion(attr.Val, &exeCtx, true)
					}
				}
			} else if tok.Type == html.EndTagToken && tok.DataAtom == atom.Script {
				if chunks != "" {
					codeHash := core.NewScriptHash(chunks)
					log.Printf("\tinsert %d bytes; sha256=%s", len(chunks), hex.EncodeToString(codeHash.SHA2[:]))
					agg.addInsertion(codeHash, &exeCtx, true)
				}
				script = nil
				chunks = ""
			} else if script != nil {
				chunks += tok.Data
			}
		}
	}

	// Emit 'include' causality links (and remove from the srcMap)
	for url, whoMap := range agg.includeMap {
		matchingScripts, ok := srcMap[url]
		if ok {
			fromCardinality := len(whoMap)
			toCardinality := len(matchingScripts)
			for _, includee := range matchingScripts {
				for includer := range whoMap {
					var gentype string
					if includer.write {
						gentype = "write_include"
					} else {
						gentype = "include"
					}
					records = append(records, causalityRecord{
						child:   includee,
						genesis: gentype,
						parent:  includer.parent,
						url:     url,
						pCard:   fromCardinality,
						cCard:   toCardinality,
					})
				}

				// Remove this one script from the codeMap
				err := removeCodeMapEntry(includee)
				if err != nil {
					log.Printf("WARNING causality: removing included script: %s", err)
				}
			}
			delete(srcMap, url) // We have processed *all* scripts under this url in srcMap
		}
	}

	// Use the remaining elements of srcMap to deduce/emit 'static' [non-]causality links
	for url, matchingScripts := range srcMap {
		for _, script := range matchingScripts {
			// Emit record
			records = append(records, causalityRecord{
				child:   script,
				genesis: "static",
				url:     url,
			})

			// Remove that script from the remaining pool
			err := removeCodeMapEntry(script)
			if err != nil {
				log.Printf("WARNING causality: static script removal: %s", err)
			}
		}
	}

	// Emit 'insert' causality links (independent of srcMap)
	for insertedHash, whoMap := range agg.insertMap {
		matchingScripts, ok := codeMap[insertedHash]
		if ok {
			fromCardinality := len(whoMap)
			toCardinality := len(matchingScripts)
			for _, insertee := range matchingScripts {
				for inserter := range whoMap {
					var gentype string
					if inserter.write {
						gentype = "write_insert"
					} else {
						gentype = "insert"
					}
					records = append(records, causalityRecord{
						child:   insertee,
						parent:  inserter.parent,
						genesis: gentype,
						pCard:   fromCardinality,
						cCard:   toCardinality,
					})
				}
			}
			delete(codeMap, insertedHash) // We have processed *all* remaining scripts under this hash
		}
	}

	// Emit all remaining/unaccounted script hashes as "unknown"
	for _, matchingScripts := range codeMap {
		for _, unknownScript := range matchingScripts {
			records = append(records, causalityRecord{
				child:   unknownScript,
				genesis: "unknown",
			})
		}
	}

	return records, nil
}

type graphmlKeyRegistration struct {
	target       graphml.KeyForElement
	name         string
	description  string
	keyType      reflect.Kind
	defaultValue interface{}
}

var graphmlKeyData = []graphmlKeyRegistration{
	{graphml.KeyForNode, "isRoot", "Is this the root node (not a script)", reflect.Bool, false},
	{graphml.KeyForNode, "isolateKey", "Unique tag for Isolate (script ID namespace)", reflect.String, "unknown"},
	{graphml.KeyForNode, "scriptID", "ID of script instance within Isolate", reflect.Int, -1},
	{graphml.KeyForNode, "bytes", "Size of script code in bytes", reflect.Int, 0},
	{graphml.KeyForNode, "sha2", "SHA-2 256 hex digest of script code contents", reflect.String, "unknown"},
	{graphml.KeyForNode, "url", "Script load URL (if any)", reflect.String, ""},
	{graphml.KeyForNode, "firstOrigin", "Active SOP origin at moment of script dumping in VV8 log", reflect.String, "unknown"},
	{graphml.KeyForEdge, "action", "Causation link type", reflect.String, "unknown"},
	{graphml.KeyForEdge, "url", "Dynamic inclusion URL (if any)", reflect.String, ""},
	{graphml.KeyForEdge, "pcard", "Parent cardinality (ambiguity signal)", reflect.Int, 0},
	{graphml.KeyForEdge, "ccard", "Child cardinality (ambiguity signal)", reflect.Int, 0},
}

// generateGraphML converts a slice of causality records (i.e., edges) into a goGraphML object that can be serialized to XML/etc.
func generateGraphML(records []causalityRecord, ctx *core.AggregationContext) (*graphml.GraphML, error) {
	gml := graphml.NewGraphML("vv8-post-processor")

	for _, keyInfo := range graphmlKeyData {
		_, err := gml.RegisterKey(keyInfo.target, keyInfo.name, keyInfo.description, keyInfo.keyType, keyInfo.defaultValue)
		if err != nil {
			log.Printf("error registering GraphML key '%s'", keyInfo.name)
			return nil, err
		}
	}

	graph, err := gml.AddGraph(fmt.Sprintf("%s", ctx.Ln.RootName), graphml.EdgeDirectionDirected, nil)
	if err != nil {
		log.Println("error creating graph object")
		return nil, err
	}

	type nodeKey struct {
		isolateKey string
		scriptID   int
	}
	nodeMap := make(map[nodeKey]*graphml.Node, 0)

	type edgeKey struct {
		parent, child *graphml.Node
	}
	edgeMap := make(map[edgeKey]*graphml.Edge)

	// Create a root node indicating ultimate causality (in theory, these were all statically included in the root document, or maybe an iframe document)
	rootNode, err := graph.AddNode(map[string]interface{}{
		"isRoot": true,
	}, "root")

	for _, r := range records {
		childNode, ok := nodeMap[nodeKey{r.child.Isolate.ID, r.child.ID}]
		if !ok {
			childNode, err = graph.AddNode(map[string]interface{}{
				"isRoot":      false,
				"isolateKey":  r.child.Isolate.ID,
				"scriptID":    r.child.ID,
				"bytes":       r.child.CodeHash.Length,
				"sha2":        hex.EncodeToString(r.child.CodeHash.SHA2[:]),
				"url":         r.child.URL,
				"firstOrigin": r.child.FirstOrigin,
			}, fmt.Sprintf("n%d", len(nodeMap)))
			if err != nil {
				log.Printf("error creating node for script %s[%d]\n", r.child.Isolate.ID, r.child.ID)
				return nil, err
			}
			nodeMap[nodeKey{r.child.Isolate.ID, r.child.ID}] = childNode
		}

		// Iff we have actual link data, figure out the parent
		if r.parent != nil {
			parentNode, ok := nodeMap[nodeKey{r.parent.Isolate.ID, r.parent.ID}]
			if !ok {
				parentNode, err = graph.AddNode(map[string]interface{}{
					"isRoot":      false,
					"isolateKey":  r.parent.Isolate.ID,
					"scriptID":    r.parent.ID,
					"bytes":       r.parent.CodeHash.Length,
					"sha2":        hex.EncodeToString(r.parent.CodeHash.SHA2[:]),
					"url":         r.parent.URL,
					"firstOrigin": r.parent.FirstOrigin,
				}, fmt.Sprintf("n%d", len(nodeMap)))
				if err != nil {
					log.Printf("error creating node for script %s[%d]\n", r.parent.Isolate.ID, r.parent.ID)
					return nil, err
				}
				nodeMap[nodeKey{r.parent.Isolate.ID, r.parent.ID}] = parentNode
			}

			edgeAttrs := map[string]interface{}{
				"action": r.genesis,
				"url":    r.url,
				"pcard":  r.pCard,
				"ccard":  r.cCard,
			}
			edge, ok := edgeMap[edgeKey{parentNode, childNode}]
			if !ok {
				edge, err = graph.AddEdge(parentNode, childNode, edgeAttrs, graphml.EdgeDirectionDefault, fmt.Sprintf("e%d", len(edgeMap)))
				if err != nil {
					log.Printf("error creating edge from %s[%d] -> %s[%d]\n", r.parent.Isolate.ID, r.parent.ID, r.child.Isolate.ID, r.child.ID)
					return nil, err
				}
				edgeMap[edgeKey{parentNode, childNode}] = edge
			} else {
				// Uh-oh, we have a problem---graphml doesn't handle multi-digraphs, so we have to report this double-edge differently
				log.Printf("Houston, we've have a problem! Double edge %s[%d] -> %s[%d]...\n", r.parent.Isolate.ID, r.parent.ID, r.child.Isolate.ID, r.child.ID)
				log.Println("LAST EDGE ATTRIBUTES:")
				oldAttrs, err := edge.GetAttributes()
				if err != nil {
					log.Println("I can't win for losing...")
					return nil, err
				}
				log.Println(oldAttrs)
				log.Println("-------------------------------------")
				log.Println("NEW EDGE ATTRIBUTES:")
				log.Println(edgeAttrs)
				log.Println("-------------------------------------")
			}
		} else {
			// Link all orphans back to root
			edge, ok := edgeMap[edgeKey{rootNode, childNode}]
			if !ok {
				edge, err = graph.AddEdge(rootNode, childNode, map[string]interface{}{
					"action": r.genesis,
				}, graphml.EdgeDirectionDefault, fmt.Sprintf("e%d", len(edgeMap)))
				edgeMap[edgeKey{rootNode, childNode}] = edge
			}

		}
	}
	return gml, nil
}

// DumpToStream serializes a ScriptCausalityAggregator's results as a GraphML XML document written to the given stream
func (agg *ScriptCausalityAggregator) DumpToStream(ctx *core.AggregationContext, stream io.Writer) error {
	records, err := agg.causalityDumper(ctx)
	if err != nil {
		log.Printf("error dumping causality tuples from raw data (%s)", err)
		return err
	}

	// new-style GraphML (XML) output (like we now store in Mongo)
	if ctx.Formats["causality_graphml"] {
		gml, err := generateGraphML(records, ctx)
		if err != nil {
			log.Printf("error converting causality tuples into goGraphML graph object (%s)", err)
			return err
		}

		err = gml.Encode(stream, true) // yes pretty-printing
		if err != nil {
			log.Printf("error serializing causality graph to GraphML (%s)", err)
			return err
		}
	}

	// old-school JSON-object causality link output (like we stored in Postgres)
	if ctx.Formats["causality"] {
		jstream := json.NewEncoder(stream)
		for _, r := range records {
			doc := core.JSONObject{
				"child_hash":         hex.EncodeToString(r.child.CodeHash.SHA2[:]),
				"genesis":            r.genesis,
				"by_url":             r.url,
				"parent_cardinality": r.pCard,
				"child_cardinality":  r.cCard,
			}
			if r.parent != nil {
				doc["parent_hash"] = hex.EncodeToString(r.parent.CodeHash.SHA2[:])
			} else {
				doc["parent_hash"] = nil
			}
			jstream.Encode(core.JSONArray{"script_causality", doc})
		}
	}

	return nil
}

var scriptCausalityFields = [...]string{
	"logfile_id",
	"visit_domain",
	"child_hash",
	"genesis",
	"parent_hash",
	"by_url",
	"parent_cardinality",
	"child_cardinality",
}

/*
// graphmlMongoOutput combines GraphML generation with upload/saving to Mongo and marking the vv8log complete for "causality_graphml"
func graphmlMongoOutput(records []causalityRecord, ctx *core.AggregationContext, mongoDb *mgo.Database) error {
	gml, err := generateGraphML(records, ctx)
	if err != nil {
		log.Printf("error converting causality tuples into goGraphML graph object (%s)", err)
		return err
	}

	buf := new(bytes.Buffer)
	err = gml.Encode(buf, false) // no pretty-printing
	if err != nil {
		log.Printf("error serializing causality graph to GraphML (%s)", err)
		return err
	}
	/*

		hash, oid, err := core.ArchiveBlob(mongoDb, fmt.Sprintf("script-causality-%s(pageId=%s).graphml", ctx.Ln.RootName, ctx.Ln.PageID), buf.Bytes(), false, nil)
		if err != nil {
			log.Printf("error saving GraphML output to MongoDB blob (%s)", err)
			return err
		}

		doc := bson.M{
			"$set": bson.M{
				"scriptCausalityGraphBlob": oid,
				"scriptCausalityGraphHash": hash,
			},
		}
		err = mongoDb.C("pages").UpdateId(ctx.Ln.PageID, doc)
		if err != nil {
			log.Printf("error linking GraphML blob hash (%s) and oid (%s) to owning page record (OID=%s) (%s)", hash, oid, ctx.Ln.PageID, err)
			return err
		}
*/ /*

	return nil
}
*/

/*
// DumpToMongo produces a causality graph and saves it as a GraphML (XML) blob to MongoDB under db.scriptGraphs[pageId=ctx.Ln.PageID]
func (agg *ScriptCausalityAggregator) DumpToMongo(ctx *core.AggregationContext, mongoDb *mgo.Database) error {
	if ctx.Formats["causality_graphml"] {
		records, err := agg.causalityDumper(ctx)
		if err != nil {
			log.Printf("error dumping causality tuples from raw data (%s)", err)
			return err
		}

		err = graphmlMongoOutput(records, ctx, mongoDb)
		if err != nil {
			log.Printf("error generating/saving GraphML (%s)", err)
			return err
		}

	}
	return nil
}
*/

// DumpToMongresql can trigger both the new GraphML-blob-save-to-Mongo logic and the old-n-busted save-links-to-Postgres logic
func (agg *ScriptCausalityAggregator) DumpToMongresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {
	records, err := agg.causalityDumper(ctx)
	if err != nil {
		return err
	}

	/*
		if ctx.Formats["causality_graphml"] {
			err = graphmlMongoOutput(records, ctx, mongoDb)
			if err != nil {
				log.Printf("error generating/saving GraphML (%s)", err)
				return err
			}
		}
	*/

	if ctx.Formats["causality"] {
		// Create log record if necessary (need job domain for that)
		visitDomain, err := core.GetRootDomain(sqlDb, ctx.Ln)
		if err != nil {
			return err
		}
		logID, err := ctx.Ln.InsertLogfile(sqlDb)
		if err != nil {
			return err
		}

		// Prepare for bulk insertion of script causality tuples
		txn, err := sqlDb.Begin()
		if err != nil {
			return err
		}
		stmt, err := txn.Prepare(pq.CopyIn("script_causality", scriptCausalityFields[:]...))
		if err != nil {
			txn.Rollback()
			return err
		}

		// Insert actual tuples
		for _, cr := range records {
			log.Printf("SCA: %p -> %p via '%s' (url: '%s') [%d/%d]", cr.parent, cr.child, cr.genesis, cr.url, cr.pCard, cr.cCard)
			var nullableParentHash interface{}
			var nullableURL interface{}
			var nullablePCard interface{}
			var nullableCCard interface{}

			if cr.parent != nil {
				nullableParentHash = cr.parent.CodeHash.SHA2[:]
			}
			if cr.url != "" {
				nullableURL = cr.url
			}
			if cr.pCard > 0 {
				nullablePCard = cr.pCard
			}
			if cr.cCard > 0 {
				nullableCCard = cr.cCard
			}
			_, err = stmt.Exec(
				logID,
				visitDomain,
				cr.child.CodeHash.SHA2[:],
				cr.genesis,
				nullableParentHash,
				nullableURL,
				nullablePCard,
				nullableCCard)
			if err != nil {
				txn.Rollback()
				return err
			}
		}

		// Finish the bulk insertion and commit everything
		_, err = stmt.Exec()
		if err != nil {
			txn.Rollback()
			return err
		}
		err = stmt.Close()
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
