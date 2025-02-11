-- WARNING: this schema depends on the `urls` tables from the VPC Postgres schema being available!
CREATE TABLE IF NOT EXISTS urls_import_schema (
    id SERIAL PRIMARY KEY NOT NULL,
    sha256 BYTEA UNIQUE NOT NULL,
    url_full TEXT,
    url_scheme TEXT,
    url_hostname TEXT,
    url_port TEXT,
    url_path TEXT,
    url_query TEXT,
    url_etld1 TEXT,
    url_stemmed TEXT
);

CREATE TABLE IF NOT EXISTS urls (
    id SERIAL PRIMARY KEY NOT NULL,
    sha256 BYTEA UNIQUE NOT NULL,
    url_full TEXT,
    url_scheme TEXT,
    url_hostname TEXT,
    url_port TEXT,
    url_path TEXT,
    url_query TEXT,
    url_etld1 TEXT,
    url_stemmed TEXT
);

-- Record of each processed log file
CREATE TABLE IF NOT EXISTS logfile (
	id SERIAL PRIMARY KEY NOT NULL,	-- PG ID for FKs from other tables
	mongo_oid BYTEA NOT NULL,	    -- Mongo vv8log OID of raw log data record
	uuid TEXT NOT NULL UNIQUE,		-- Unique UUID for this log file
	root_name TEXT NOT NULL,		-- Root name of log file as originally stored (prefix of all segment names)
	size BIGINT NOT NULL,			-- Aggregate size (bytes) of all log segments processed
	lines INT NOT NULL,				-- Aggregate size (lines) of all log segments processed
	submissionid TEXT		-- Submission ID of the log file
);

-- Record of each distinct script body loaded
CREATE TABLE IF NOT EXISTS mega_scripts (
    id SERIAL PRIMARY KEY NOT NULL,
    sha2 BYTEA NOT NULL,
    sha3 BYTEA NOT NULL,
    size INT NOT NULL,
    UNIQUE (sha2, sha3, size)   -- Terminate script duplication with extreme prejudice
    -- TODO: expand with lexical/AST hashing here??
);

CREATE TABLE IF NOT EXISTS mega_scripts_import_schema (
    sha2 BYTEA NOT NULL,
    sha3 BYTEA NOT NULL,
    size INT NOT NULL,
    PRIMARY KEY (sha2, sha3, size)
);

-- Record of each _instance_ when a given script was loaded
CREATE TABLE IF NOT EXISTS mega_instances (
    id SERIAL PRIMARY KEY NOT NULL,
    instance_hash BYTEA UNIQUE NOT NULL,                    -- Hash of (log's mongo_oid + script-sha2 + script-sha3 + script-size + isolate-ptr + runtime-id)
    logfile_id INT REFERENCES logfile(id),                  -- Log file from which observed
    script_id INT REFERENCES mega_scripts(id),              -- Script body loaded
    isolate_ptr TEXT NOT NULL,                              -- Isolate pointer (hex string) for this execution context
    runtime_id INT NOT NULL,                                -- V8 runtime ID of this script
    origin_url_id INT REFERENCES urls(id),                  -- Origin URL active at time of script load (if available) [`urls` from VPC!]
    script_url_id INT REFERENCES urls(id),                  -- Script-load URL (if available) [`urls` from VPC!]
    eval_parent_hash BYTEA                                  -- Psuedo-self-FK to parent-instant (in the case of eval chains); uses hash rather than ID to simplify import
);

CREATE TABLE IF NOT EXISTS mega_instances_import_schema (
    instance_hash BYTEA UNIQUE NOT NULL,
    logfile_id INT REFERENCES logfile(id),
    script_id INT REFERENCES mega_scripts(id),
    isolate_ptr TEXT NOT NULL,
    runtime_id INT NOT NULL,
    origin_url_sha256 BYTEA,
    script_url_sha256 BYTEA,
    eval_parent_hash BYTEA
);

-- Record of each distinct VV8 "feature" observed
CREATE TABLE IF NOT EXISTS mega_features (
    id SERIAL PRIMARY KEY NOT NULL,
    sha256 BYTEA UNIQUE NOT NULL,           -- sha256(full_name) used for indexing/deduping
    full_name TEXT NOT NULL,                -- Original receiver/member name as produced by VV8 (with some filtering/cleanup)
    receiver_name TEXT,                     -- Split-out receiver name (if available)
    member_name TEXT,                       -- Split-out member name (if available)
    idl_base_receiver TEXT,                 -- IDL-defined base-interface (if available; e.g., "Node" for "HTMLIFrameElement")
    idl_member_role CHAR(1)                 -- IDL-defined member role ('p' property, 'm' method, 'c' constructor, '?' unknown/error)
);

CREATE TABLE IF NOT EXISTS mega_features_import_schema (
    sha256 BYTEA PRIMARY KEY,
    full_name TEXT NOT NULL,
    receiver_name TEXT,
    member_name TEXT,
    idl_base_receiver TEXT,
    idl_member_role CHAR(1)
);

-- Record of aggregate feature-usage-per-callsite-per-script-instance
CREATE TABLE IF NOT EXISTS mega_usages (
    instance_id INT REFERENCES mega_instances(id) NOT NULL,     -- Which script-instance was active?
    feature_id INT REFERENCES mega_features(id) NOT NULL,       -- Which feature was accessed?
    origin_url_id INT REFERENCES urls(id),			-- Optional execution-context-origin URL
    usage_offset INT NOT NULL,                                  -- Where in the script (byte offset)?
    usage_mode CHAR(1) NOT NULL,                                -- How? ('g' get, 's' set, 'c' call, 'n' constructor-call)
    usage_count INT NOT NULL,                                   -- Aggregate count of these uses
    PRIMARY KEY (instance_id, feature_id, origin_url_id, usage_offset, usage_mode)
);

CREATE TABLE IF NOT EXISTS mega_usages_import_schema (
    instance_id INT REFERENCES mega_instances(id) NOT NULL,
    feature_id INT REFERENCES mega_features(id) NOT NULL,
    origin_url_sha256 BYTEA,
    usage_offset INT NOT NULL,
    usage_mode CHAR(1) NOT NULL,
    usage_count INT NOT NULL,
    PRIMARY KEY (instance_id, feature_id, origin_url_sha256, usage_offset, usage_mode)
);

CREATE TABLE IF NOT EXISTS script_blobs (
	id SERIAL PRIMARY KEY NOT NULL,
	script_hash BYTEA NOT NULL,
	script_code TEXT NOT NULL,
	sha256sum BYTEA NOT NULL,
	size INT NOT NULL
);

CREATE TABLE IF NOT EXISTS adblock (
	id SERIAL PRIMARY KEY NOT NULL,
	url TEXT NOT NULL,
	origin TEXT NOT NULL,
	blocked BOOLEAN NOT NULL -- If the url,origin was blocked by brave adblock (using easylist and easyprivacy)
);

CREATE TABLE IF NOT EXISTS thirdpartyfirstparty (
	id SERIAL PRIMARY KEY NOT NULL,
	sha2 BYTEA NOT NULL,										-- SHA256 of script code
	root_domain TEXT NOT NULL, 									-- Root domain (the initial URL being loaded by pupeteer) of script if availiable
	url TEXT NOT NULL,											-- URL of script if availiable
	first_origin TEXT NOT NULL, 								-- First origin in which the script was loaded if availiable
	property_of_root_domain TEXT NOT NULL,						-- Tracker radar "property" (company name) of root domain if availiable, else eTLD+1 of the URL
	property_of_first_origin TEXT NOT NULL,						-- Tracker radar "property" (company name) of first origin if availiable, else eTLD+1 of the URL
	property_of_script TEXT NOT NULL,							-- Tracker radar "property" (company name) of script if availiable, else eTLD+1 of the URL
	is_script_third_party_with_root_domain BOOLEAN NOT NULL,	-- Is the script third party with respect to the root domain?
	is_script_third_party_with_first_origin BOOLEAN NOT NULL,   -- Is the script third party with respect to the first origin in which it was loaded?
	script_origin_tracking_value double precision NOT NULL      -- Tracking value as assigned by duckduckgo tracking radar
);

CREATE TABLE IF NOT EXISTS xleaks (
	id SERIAL PRIMARY KEY NOT NULL,
	isolate TEXT NOT NULL,
	visiblev8 BOOLEAN NOT NULL,
	first_origin TEXT,
	url TEXT,
	evaled_by INT -- REFERENCES script_flow (id)
);

CREATE TABLE IF NOT EXISTS js_api_features_summary (
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	all_features JSON NOT NULL
);

CREATE TABLE IF NOT EXISTS script_flow (
	id SERIAL PRIMARY KEY NOT NULL,
	isolate TEXT NOT NULL, -- V8 isolate pointer
	visiblev8 BOOLEAN NOT NULL, -- Is the script loaded by the browser/injected by VisibleV8 (in most cases you want to ignore scripts if this is true)
	code TEXT NOT NULL,
	sha256 BYTEA,
	first_origin TEXT,
	url TEXT,
	apis TEXT[] NOT NULL,	-- All APIs loaded by a script in the order they were executed
	evaled_by INT -- REFERENCES script_flow (id)
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

CREATE TABLE IF NOT EXISTS multi_origin_obj (
	id SERIAL PRIMARY KEY NOT NULL,
	objectid SERIAL NOT NULL,
	origins TEXT[] NOT NULL,
	num_of_origins INT NOT NULL,
	urls TEXT[] NOT NULL
);

CREATE TABLE IF NOT EXISTS multi_origin_api_names (
	id SERIAL PRIMARY KEY NOT NULL,
	objectid SERIAL NOT NULL,
	origin TEXT NOT NULL,
	api_name TEXT NOT NULL
);

-- Script creation records (only URL/eval causality included)
CREATE TABLE IF NOT EXISTS script_creation (
	id SERIAL PRIMARY KEY NOT NULL,
	isolate_ptr TEXT, -- V8 isolate pointer
	logfile_id INT REFERENCES logfile (id) NOT NULL,
	visit_domain TEXT NOT NULL,
	script_hash BYTEA NOT NULL,
	script_url TEXT,
	runtime_id INT,
	first_origin TEXT,
	eval_parent_hash BYTEA
);

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
