-- Record of each processed log file
CREATE TABLE IF NOT EXISTS logfile (
	id SERIAL PRIMARY KEY NOT NULL,	-- PG ID for FKs from other tables
	mongo_id BYTEA NOT NULL,	-- Mongo vv8log OID of raw log data record
	uuid TEXT NOT NULL UNIQUE,		-- Unique UUID for this log file
	job_id TEXT,					-- Associated job tag/id (IF KNOWN)
	run_mongo_id BYTEA, 			-- Associated Mongo run.start OID (IF KNOWN)
	root_name TEXT NOT NULL,		-- Root name of log file as originally stored (prefix of all segment names)
	size BIGINT NOT NULL,			-- Aggregate size (bytes) of all log segments processed
	lines INT NOT NULL				-- Aggregate size (lines) of all log segments processed
);

CREATE TABLE IF NOT EXISTS script_blobs (
	id SERIAL PRIMARY KEY NOT NULL,
	script_hash BYTEA NOT NULL,
	script_code TEXT NOT NULL,
	sha256sum BYTEA NOT NULL,
	size INT NOT NULL
);

CREATE TABLE IF NOT EXISTS script_flow (
	id INT PRIMARY KEY NOT NULL,
	isolate TEXT NOT NULL,
	visiblev8 BOOLEAN NOT NULL,
	code TEXT NOT NULL,
	first_origin TEXT,
	url TEXT,
	apis TEXT[],
	evaled_by INT -- REFERENCES script_flow (id)
);

CREATE TABLE IF NOT EXISTS js_api_features_summary (
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	all_features JSON NOT NULL
);

-- Feature usage information (for monomorphic callsites)
CREATE TABLE IF NOT EXISTS feature_usage (
	id SERIAL PRIMARY KEY NOT NULL,
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	visit_domain TEXT NOT NULL,
	security_origin TEXT NOT NULL,
	script_hash BYTEA NOT NULL,
	script_offset INT NOT NULL,
	feature_name TEXT NOT NULL,
	feature_use CHAR NOT NULL,
	use_count INT NOT NULL
);

-- Script creation records (only URL/eval causality included)
CREATE TABLE IF NOT EXISTS script_creation (
	id SERIAL PRIMARY KEY NOT NULL,
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	visit_domain TEXT NOT NULL,
	script_hash BYTEA NOT NULL,
	script_url TEXT,
	eval_parent_hash BYTEA
);

-- UPGRADES: adding V8's internal runtime_id and isolate_ptr
ALTER TABLE IF EXISTS script_creation ADD COLUMN IF NOT EXISTS isolate_ptr TEXT;
ALTER TABLE IF EXISTS script_creation ADD COLUMN IF NOT EXISTS runtime_id INT;

-- UPGRADES: adding the active origin at time of script dumping in the log
ALTER TABLE IF EXISTS script_creation ADD COLUMN IF NOT EXISTS first_origin TEXT;

-- Feature usage information (for polymorphic callsites)
CREATE TABLE IF NOT EXISTS poly_feature_usage (
	id SERIAL PRIMARY KEY NOT NULL,
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	visit_domain TEXT NOT NULL,
	security_origin TEXT NOT NULL,
	script_hash BYTEA NOT NULL,
	script_offset INT NOT NULL,
	feature_name TEXT NOT NULL,
	feature_use CHAR NOT NULL,
	use_count INT NOT NULL
);

-- Script causality/provenance enum type
CREATE TYPE script_genesis AS ENUM (
	'unknown',         -- No pattern (or multiple ambiguous patterns) match genesis data
	'static',          -- No parent, URL provided (appears to be loaded in document HTML [of some frame])
	'eval',            -- Eval-parent (redundant/overlap with script_creation, but that's life)
	'include',         -- Direct HTMLScriptElement.src manipulation matches subsequent URL-load of script
	'insert',          -- Direct HTMLScriptElement.text(et al.) manipulation matches SHA256 of subsequent non-URL-load of script
	'write_include',   -- Document.write-injected <script src="..." /> that matches subsequent URL-load of script
	'write_insert');   -- Document.write-injected <script>...</script> that matches SHA256 of subsequent non-URL-load of script

-- Script causality/provenance link data
CREATE TABLE IF NOT EXISTS script_causality (
	id SERIAL PRIMARY KEY NOT NULL,
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	visit_domain TEXT NOT NULL,
	child_hash BYTEA NOT NULL,
	genesis script_genesis NOT NULL DEFAULT 'unknown',
	parent_hash BYTEA,
	by_url TEXT,
	parent_cardinality INT,
	child_cardinality INT
);

-- Source/origin of document.createElement calls
CREATE TABLE IF NOT EXISTS create_elements (
	id SERIAL PRIMARY KEY NOT NULL,
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	visit_domain TEXT NOT NULL,
	security_origin TEXT NOT NULL,
	script_hash BYTEA NOT NULL,
	script_offset INT NOT NULL,
	tag_name TEXT NOT NULL,
	create_count INT NOT NULL
);

-- [VPC-specific] table of page/logfile/{set-of-detected-captchas} records
CREATE TABLE IF NOT EXISTS page_captcha_systems (
	id SERIAL PRIMARY KEY NOT NULL,
	page_mongo_oid BYTEA NOT NULL,
	logfile_mongo_oid BYTEA NOT NULL,
	captcha_systems JSONB NOT NULL
);
