package mega

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/wspr-ncsu/visiblev8/post-processor/core"
)

// scriptHashIDMap is used to map the global script body identifiers to Postgres FK IDs
type scriptHashIDMap map[core.ScriptHash]int

// featureNameMap is used to map distinct feature names to Postgres FK IDs
type featureNameMap map[string]int

// instanceMetaMap is used to map distinct ScriptInfo objects to Postgres FK IDs
type instanceMetaMap map[*core.ScriptInfo]int

// mongresqlContext tracks the { item->FK ID } mappings built up over each step of the import process for use by later stages
type postgresqlContext struct {
	logfileID   int             // the logfile all this stuff comes from (mega_logfile.id FK)
	hashMap     scriptHashIDMap // map script hashes (SHA2/SHA3/Size) -> mega_scripts.id FK
	instanceMap instanceMetaMap // map script instances (ptr to ScriptInfo) -> mega_instances.id FK
	featureMap  featureNameMap  // map full feature names -> mega_features.id FK
}

// DumpToMongresql handles bulk-insert of activity records into Postgres (we don't use MongoDB for any output)
func (agg *usageAggregator) DumpToPostgresql(ctx *core.AggregationContext, sqlDb *sql.DB) error {
	var err error
	var pctx postgresqlContext

	// Step 0: Make sure the current log file is inserted (and get its ID)
	pctx.logfileID, err = ctx.Ln.InsertLogfile(sqlDb)
	if err != nil {
		return fmt.Errorf("megaFeatures.DumpToMongresql/logFile: %w", err)
	}

	// Step 1: import the new script-body-hashes and build a hash->ID mapping for subsequent phases
	if err = pctx.sqlDumpScriptHashes(sqlDb, ctx.Ln); err != nil {
		return fmt.Errorf("megaFeatures.DumpToMongresql/scriptHashes: %w", err)
	}

	// Step 2: import all loaded-instances of the scripts and build a scriptInfo->ID mapping for subsequent phases
	if err = pctx.sqlDumpScriptInstances(sqlDb, ctx.Ln); err != nil {
		return fmt.Errorf("megaFeatures.DumpToMongresql/scriptInstances: %w", err)
	}

	// Step 3: import the new distinct feature names (and metadata) and build a name->ID mapping for subsequent phases
	if err = pctx.sqlDumpDistinctFeatures(sqlDb, agg); err != nil {
		return fmt.Errorf("megaFeatures.DumpToMongresql/distinctFeatures: %w", err)
	}

	// Step 4: import the aggregated usage counts (referencing features and instances/scripts)
	if err = pctx.sqlDumpUsageCounts(sqlDb, agg); err != nil {
		return fmt.Errorf("megaFeatures.DumpToMongresql/usageCounts: %w", err)
	}
	log.Printf("Mfeatures.DumpToMongresql: done.")
	return nil
}

var scriptHashImportFields = [...]string{
	"sha2",
	"sha3",
	"size",
}

func (pctx *postgresqlContext) sqlDumpScriptHashes(sqlDb *sql.DB, ln *core.LogInfo) error {
	// Step 1a: compute/bulk-insert the set of distinct script [hashes] loaded into an import table
	//---------------------------------------------------------------------------------------------
	log.Printf("Mfeatures.sqlDumpScriptHashes: creating temp table 'import_scripts'...")
	if err := core.CreateImportTable(sqlDb, "mega_scripts_import_schema", "import_scripts"); err != nil {
		return err
	}
	defer func() {
		log.Printf("Mfeature.sqlDumpScriptHashes: dropping temp table 'import_scripts'...")
		_, err := sqlDb.Exec("DROP TABLE import_scripts;")
		if err != nil {
			log.Printf("Mfeatures.sqlDumpScriptHashes: failed to drop `import_scripts` temp table (%v)\n", err)
		}
	}()

	log.Printf("Mfeatures.sqlDumpScriptHashes: computing set of distinct script-hashes encountered in logfileID=%d\n...", pctx.logfileID)
	if pctx.hashMap == nil {
		pctx.hashMap = make(scriptHashIDMap)
	}
	for _, iso := range ln.Isolates {
		for _, script := range iso.Scripts {
			pctx.hashMap[script.CodeHash] = 0 // placeholder: later to be replaced by Postgres FK ID
		}
	}
	hashChan := make(chan core.ScriptHash)
	go func() {
		for shash := range pctx.hashMap {
			hashChan <- shash
		}
		close(hashChan)
	}()
	log.Printf("Mfeatures.sqlDumpScriptHashes: bulk-inserting...")
	importRows, err := core.BulkInsertRows(
		sqlDb, "MFeatures.sqlDumpScriptHashes", "import_scripts",
		scriptHashImportFields[:],
		func() ([]interface{}, error) {
			shash, ok := <-hashChan
			if !ok {
				return nil, nil // end-of-stream
			}

			values := []interface{}{
				shash.SHA2[:],
				shash.SHA3[:],
				shash.Length,
			}
			return values, nil
		})
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpScriptHashes: bulk-inserted %d distinct rows", importRows)

	// Step 1b: copy-insert into the permanent script-body table (upsert; dropping dups)
	//----------------------------------------------------------------------------------
	tx, err := sqlDb.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			log.Println("Mfeatures.sqlDumpScriptHashes: rolling back copy-upsert")
			if err := tx.Rollback(); err != nil {
				log.Printf("Mfeatures.sqlDumpScriptHashes: copy-upsert rollback error (%v)", err)
			}
		}
	}()

	// Script hashes are shared in common across all logs; concurrent upsert can lead to deadlock; GO NUCLEAR and lock the table
	// (auto released on transaction commit/rollback)
	if _, err = tx.Exec(`LOCK TABLE mega_scripts IN SHARE ROW EXCLUSIVE MODE;`); err != nil {
		return err
	}

	log.Printf("Mfeatures.sqlDumpScriptHashes: copy-upserting into permanent table...")
	copyResult, err := tx.Exec(`
INSERT INTO mega_scripts (sha2, sha3, size)
	SELECT ish.sha2, ish.sha3, ish.size
	FROM import_scripts AS ish
ON CONFLICT DO NOTHING;
`)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	tx = nil // No more deferred-rollback (we're committed!)

	insertRows, err := copyResult.RowsAffected()
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpScriptHashes: inserted %d (out of %d) import rows\n", insertRows, importRows)

	// Step 1c: lookup permanent IDs of all scripts in the import table before dropping (retain the mapping)
	//------------------------------------------------------------------------------------------------------
	lookupRows, err := sqlDb.Query(`
		SELECT id, sha2, sha3, size
		FROM mega_scripts AS ms
		INNER JOIN import_scripts AS ish USING (sha2, sha3, size)
	`)
	if err != nil {
		return err
	}
	for lookupRows.Next() {
		var sid int
		var shash core.ScriptHash
		var sha2Slice, sha3Slice []byte
		if err = lookupRows.Scan(&sid, &sha2Slice, &sha3Slice, &shash.Length); err != nil {
			return err
		}
		copy(shash.SHA2[:], sha2Slice)
		copy(shash.SHA3[:], sha3Slice)
		pctx.hashMap[shash] = sid
	}

	return nil
}

type reverseInstanceKey [sha256.Size]byte
type reverseInstanceMetaMap map[reverseInstanceKey]*core.ScriptInfo

// hashInstance derives a natural key via SHA256(log-oid + script-sha2 + script-sha3 + script-size + isolate-ptr + runtime-id)
func hashInstance(ln *core.LogInfo, instance *core.ScriptInfo) (reverseInstanceKey, error) {
	var digest reverseInstanceKey
	stomach := sha256.New()

	things := []interface{}{
		[]byte(ln.ID.String()),
		instance.CodeHash.SHA2[:],
		instance.CodeHash.SHA3[:],
		int64(instance.CodeHash.Length),
		[]byte(instance.Isolate.ID),
		int64(instance.ID),
	}
	for _, thing := range things {
		if err := binary.Write(stomach, binary.LittleEndian, thing); err != nil {
			return digest, err
		}
	}

	stomach.Sum(digest[:0])
	return digest, nil
}

var instanceImportFields = [...]string{
	"instance_hash",
	"logfile_id",
	"script_id",
	"isolate_ptr",
	"runtime_id",
	"origin_url_sha256",
	"script_url_sha256",
	"eval_parent_hash",
}

func (pctx *postgresqlContext) sqlDumpScriptInstances(sqlDb *sql.DB, ln *core.LogInfo) error {
	// Step 2a: bulk-insert into import table (generating instance-hashes along the way)
	//----------------------------------------------------------------------------------
	log.Printf("Mfeatures.sqlDumpScriptInstances: creating temp table 'import_instances'...")
	if err := core.CreateImportTable(sqlDb, "mega_instances_import_schema", "import_instances"); err != nil {
		return err
	}
	defer func() {
		log.Printf("Mfeature.sqlDumpScriptInstances: dropping temp table 'import_instances'...")
		_, err := sqlDb.Exec("DROP TABLE import_instances;")
		if err != nil {
			log.Printf("Mfeatures.sqlDumpScriptInstances: failed to drop `import_instances` temp table (%v)\n", err)
		}
	}()

	scriptChan := make(chan *core.ScriptInfo)
	go func() {
		for _, iso := range ln.Isolates {
			for _, script := range iso.Scripts {
				scriptChan <- script
			}
		}
		close(scriptChan)
	}()

	ub := core.NewURLBakery()
	rimap := make(reverseInstanceMetaMap)
	importRows, err := core.BulkInsertRows(
		sqlDb, "MFeatures.sqlDumpScriptInstances", "import_instances",
		instanceImportFields[:],
		func() ([]interface{}, error) {
			script, ok := <-scriptChan
			if !ok {
				return nil, nil // end-of-stream
			}

			instaHash, err := hashInstance(ln, script)
			if err != nil {
				return nil, err
			}
			var evalParentHash interface{}
			if script.EvaledBy != nil {
				var temp reverseInstanceKey
				temp, err = hashInstance(ln, script.EvaledBy)
				if err != nil {
					return nil, err
				}
				evalParentHash = temp[:]
			}

			var originURLHash, scriptURLHash interface{}
			if script.FirstOrigin.Origin != "" {
				temp := ub.URLToHash(script.FirstOrigin.Origin)
				originURLHash = temp[:]
			}
			if script.URL != "" {
				temp := ub.URLToHash(script.URL)
				scriptURLHash = temp[:]
			}
			sid := pctx.hashMap[script.CodeHash]

			values := []interface{}{
				instaHash[:],
				pctx.logfileID,
				sid,
				script.Isolate.ID,
				script.ID,
				originURLHash,
				scriptURLHash,
				evalParentHash,
			}
			rimap[instaHash] = script

			return values, nil
		})
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpScriptInstances: bulk-inserted %d distinct rows", importRows)

	// Step 2a[ii]: bulk-insert the processed URLs we used (if any)
	if err = ub.InsertBakedURLs(sqlDb); err != nil {
		return err
	}

	// Step 2b: copy-insert import data into permanent table (upsert)
	//---------------------------------------------------------------------------------
	log.Printf("Mfeatures.sqlDumpScriptInstances: copy-upserting into permanent table...")
	copyResult, err := sqlDb.Exec(`
INSERT INTO mega_instances(
		instance_hash, logfile_id, script_id, isolate_ptr, runtime_id,
		origin_url_id, script_url_id, eval_parent_hash)
	SELECT
		ii.instance_hash, ii.logfile_id, ii.script_id, ii.isolate_ptr, ii.runtime_id,
		ouu.id, suu.id, ii.eval_parent_hash
	FROM import_instances AS ii
		LEFT JOIN urls AS ouu ON (ouu.sha256 = ii.origin_url_sha256)
		LEFT JOIN urls AS suu ON (suu.sha256 = ii.script_url_sha256)
ON CONFLICT DO NOTHING
`)
	if err != nil {
		return err
	}
	insertRows, err := copyResult.RowsAffected()
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpScriptInstances: inserted %d (out of %d) import rows\n", insertRows, importRows)

	// Step 2c: lookup permanent IDs of all instances in the import table before dropping (retain the mapping)
	//--------------------------------------------------------------------------------------------------------
	lookupRows, err := sqlDb.Query(`
SELECT id, instance_hash
FROM mega_instances AS mi
	INNER JOIN import_instances AS ii USING (instance_hash)
`)
	if err != nil {
		return err
	}
	if pctx.instanceMap == nil {
		pctx.instanceMap = make(instanceMetaMap)
	}
	for lookupRows.Next() {
		var instanceID int
		var hashSlice []byte
		if err = lookupRows.Scan(&instanceID, &hashSlice); err != nil {
			return err
		}
		var hashArray reverseInstanceKey
		copy(hashArray[:], hashSlice)
		instance := rimap[hashArray]
		pctx.instanceMap[instance] = instanceID
	}
	if err = lookupRows.Err(); err != nil {
		return err
	}

	return nil
}

var featureImportFields = [...]string{
	"sha256",
	"full_name",
	"receiver_name",
	"member_name",
	"idl_base_receiver",
	"idl_member_role",
}

func (pctx *postgresqlContext) sqlDumpDistinctFeatures(sqlDb *sql.DB, agg *usageAggregator) error {
	// Step 3a: bulk-insert the set of distinct feature [names] observed
	//------------------------------------------------------------------
	log.Printf("Mfeatures.sqlDumpDistinctFeatures: creating temp table 'import_features'...")
	if err := core.CreateImportTable(sqlDb, "mega_features_import_schema", "import_features"); err != nil {
		return err
	}
	defer func() {
		log.Printf("Mfeature.sqlDumpDistinctFeatures: dropping temp table 'import_features'...")
		_, err := sqlDb.Exec("DROP TABLE import_features;")
		if err != nil {
			log.Printf("Mfeatures.sqlDumpDistinctFeatures: failed to drop `import_features` temp table (%v)\n", err)
		}
	}()
	featureChan := make(chan *Feature)
	go func() {
		for _, feature := range agg.features {
			featureChan <- feature
		}
		close(featureChan)
	}()
	log.Printf("Mfeatures.sqlDumpDistinctFeatures: bulk-inserting...")
	importRows, err := core.BulkInsertRows(
		sqlDb, "MFeatures.sqlDumpDistinctFeatures", "import_features",
		featureImportFields[:],
		func() ([]interface{}, error) {
			feature, ok := <-featureChan
			if !ok {
				return nil, nil // end-of-stream
			}

			featureHash := sha256.Sum256([]byte(feature.fullName))
			values := []interface{}{
				featureHash[:],
				feature.fullName,
				core.NullableString(feature.receiverName),
				core.NullableString(feature.memberName),
				core.NullableString(feature.idlInfo.BaseInterface),
				core.NullableRune(feature.idlInfo.MemberRole),
			}
			return values, nil
		})
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpDistinctFeatures: bulk-inserted %d distinct rows", importRows)

	// Step 3b: copy-insert into the permanent feature table (upsert; dropping dups [make serializable because of overlap between concurrent logs])
	//---------------------------------------------------------------------------------------------------------------------------------------------
	log.Printf("Mfeatures.sqlDumpDistinctFeatures: copy-upserting into permanent table...")
	tx, err := sqlDb.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			log.Println("Mfeatures.sqlDumpDistinctFeatures: rolling back copy-upsert...")
			if err := tx.Rollback(); err != nil {
				log.Printf("Mfeatures.sqlDumpDistinctFeatures: copy-upsert rollback error (%v)", err)
			}
		}
	}()

	// Features are shared in common across all logs; concurrent upsert can lead to deadlock; GO NUCLEAR and lock the table
	// (auto released on transaction commit/rollback)
	if _, err = tx.Exec(`LOCK TABLE mega_features IN SHARE ROW EXCLUSIVE MODE;`); err != nil {
		return err
	}

	copyResult, err := tx.Exec(`
INSERT INTO mega_features (
		sha256, full_name, receiver_name, member_name,
		idl_base_receiver, idl_member_role)
	SELECT
		imf.sha256, imf.full_name, imf.receiver_name, imf.member_name,
		imf.idl_base_receiver, imf.idl_member_role
	FROM import_features AS imf
ON CONFLICT DO NOTHING;
`)
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// Transaction committed; no need to deferred rollback any longer
	tx = nil

	insertRows, err := copyResult.RowsAffected()
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpDistinctFeatures: inserted %d (out of %d) import rows\n", insertRows, importRows)

	// Step 3c: lookup permanent IDs of all features in the import table before dropping (retain the mapping)
	//-------------------------------------------------------------------------------------------------------
	lookupRows, err := sqlDb.Query(`
SELECT mf.id, mf.full_name
FROM mega_features AS mf
	INNER JOIN import_features AS imf USING (full_name)
`)
	if err != nil {
		return err
	}
	if pctx.featureMap == nil {
		pctx.featureMap = make(featureNameMap)
	}
	for lookupRows.Next() {
		var fid int
		var name string
		if err = lookupRows.Scan(&fid, &name); err != nil {
			return err
		}
		pctx.featureMap[name] = fid
	}
	return nil
}

var usageImportFields = [...]string{
	"instance_id",
	"feature_id",
	"origin_url_sha256",
	"usage_offset",
	"usage_mode",
	"usage_count",
}

func (pctx *postgresqlContext) sqlDumpUsageCounts(sqlDb *sql.DB, agg *usageAggregator) error {
	// Step 4a. Insert raw tuples (with URL hashes) into temp import table
	log.Printf("Mfeatures.sqlDumpUsageCounts: creating temp table 'import_usages'...")
	if err := core.CreateImportTable(sqlDb, "mega_usages_import_schema", "import_usages"); err != nil {
		return err
	}
	defer func() {
		log.Printf("Mfeature.sqlDumpUsageCounts: dropping temp table 'import_usages'...")
		_, err := sqlDb.Exec("DROP TABLE import_usages;")
		if err != nil {
			log.Printf("Mfeatures.sqlDumpUsageCounts: failed to drop `import_usages` temp table (%v)\n", err)
		}
	}()
	usageChan := make(chan Usage)
	go func() {
		for usage := range agg.usageCounts {
			usageChan <- usage
		}
		close(usageChan)
	}()
	ub := core.NewURLBakery()
	log.Printf("Mfeatures.sqlDumpUsageCounts: bulk-inserting...")
	importRows, err := core.BulkInsertRows(
		sqlDb, "MFeatures.sqlDumpUsageCounts", "import_usages",
		usageImportFields[:],
		func() ([]interface{}, error) {
			usage, ok := <-usageChan
			if !ok {
				return nil, nil // end-of-stream
			}

			instanceID := pctx.instanceMap[usage.script]
			featureID := pctx.featureMap[usage.feature.fullName]
			originURLHash := ub.URLToHash(usage.origin)
			values := []interface{}{
				instanceID,
				featureID,
				originURLHash[:],
				usage.offset,
				core.NullableRune(usage.mode),
				agg.usageCounts[usage],
			}
			return values, nil
		})
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpUsageCounts: bulk-inserted %d rows", importRows)

	// Step 4a[ii]: bulk-insert the processed URLs we used (if any)
	if err = ub.InsertBakedURLs(sqlDb); err != nil {
		return err
	}

	// Step 4b: copy-insert into the permanent usage table (upsert; dropping dups)
	//------------------------------------------------------------------------------
	log.Printf("Mfeatures.sqlDumpDistinctUsages: copy-upserting into permanent table...")
	copyResult, err := sqlDb.Exec(`
INSERT INTO mega_usages (
		instance_id, feature_id, origin_url_id,
		usage_offset, usage_mode, usage_count)
	SELECT
		instance_id, feature_id, ou.id,
		usage_offset, usage_mode, usage_count
	FROM import_usages AS imu
		LEFT JOIN urls AS ou ON (ou.sha256 = imu.origin_url_sha256)
ON CONFLICT DO NOTHING;
`)
	if err != nil {
		return err
	}
	insertRows, err := copyResult.RowsAffected()
	if err != nil {
		return err
	}
	log.Printf("Mfeatures.sqlDumpDistinctUsages: inserted %d (out of %d) import rows\n", insertRows, importRows)

	return nil
}
