-- WARNING: this schema depends on the `urls` tables from the VPC Postgres schema being available!

-- Record of each processed log file
CREATE TABLE IF NOT EXISTS mega_logfile (
    id SERIAL PRIMARY KEY NOT NULL,     -- PG ID for FKs from other tables
    mongo_oid BYTEA UNIQUE NOT NULL,    -- Mongo vv8log OID of raw log data record
    root_name TEXT NOT NULL,            -- Root name of log file as originally stored (prefix of all segment names)
    size BIGINT NOT NULL,               -- Aggregate size (bytes) of all log segments processed
    lines INT NOT NULL,                 -- Aggregate size (lines) of all log segments processed
    page_oid BYTEA                      -- Associate page Mongo OID (IF KNOWN)
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
    logfile_id INT REFERENCES mega_logfile(id) NOT NULL,    -- Log file from which observed
    script_id INT REFERENCES mega_scripts(id) NOT NULL,     -- Script body loaded
    isolate_ptr TEXT NOT NULL,                              -- Isolate pointer (hex string) for this execution context
    runtime_id INT NOT NULL,                                -- V8 runtime ID of this script
    origin_url_id INT REFERENCES urls(id),                  -- Origin URL active at time of script load (if available) [`urls` from VPC!]
    script_url_id INT REFERENCES urls(id),                  -- Script-load URL (if available) [`urls` from VPC!]
    eval_parent_hash BYTEA                                  -- Psuedo-self-FK to parent-instant (in the case of eval chains); uses hash rather than ID to simplify import
);

CREATE TABLE IF NOT EXISTS mega_instances_import_schema (
    instance_hash BYTEA UNIQUE NOT NULL,
    logfile_id INT REFERENCES mega_logfile(id) NOT NULL,
    script_id INT REFERENCES mega_scripts(id) NOT NULL,
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
