package core

// utilities for I/O to Mongo collections used by vv8/carburetor:
// * get-blob, store-blob
// * get-job-domain, update-job-status
// * stream/decompress multi-part log blobs through a Reader interface

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"time"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// getDialURL constructs (from environment tuning variables) a MongoDB connection URL
func getDialURL() string {
	host := GetEnvDefault("MONGODB_HOST", "localhost")
	port := GetEnvDefault("MONGODB_PORT", "27017")
	db := GetEnvDefault("MONGODB_DB", "test")
	return fmt.Sprintf("mongodb://%s:%s/%s", host, port, db)
}

// MongoConnection bundles together essential state and stats for a live MongoDB connection
type MongoConnection struct {
	URL     string       // connection URL dialed ("mongodb://$host:$port/$db")
	User    string       // username authenticated as (if applicable; "" otherwise)
	Session *mgo.Session // Active (and possibly authenticated) session to Mongo
}

func (mc MongoConnection) String() string {
	var active, auth string
	if mc.Session != nil {
		active = " (ACTIVE)"
	}
	if mc.User != "" {
		auth = fmt.Sprintf(" as %s", mc.User)
	}
	return fmt.Sprintf("%s%s%s", mc.URL, auth, active)
}

// DialMongo creates a possibly authenticated Mongo session based on ENV configs
// Returns a MongoConnection on success; error otherwise (in all cases, the `url` field of MongoConnection will be set)
func DialMongo() (MongoConnection, error) {
	var conn MongoConnection
	conn.URL = getDialURL()
	session, err := mgo.Dial(conn.URL)
	if err != nil {
		return conn, err
	}

	conn.User = GetEnvDefault("MONGODB_USER", "")
	if conn.User != "" {
		creds := mgo.Credential{Username: conn.User}
		creds.Password = GetEnvDefault("MONGODB_PWD", "")
		creds.Source = GetEnvDefault("MONGODB_AUTHDB", "admin")
		err = session.Login(&creds)
		if err != nil {
			session.Close()
			return conn, err
		}
	}
	conn.Session = session
	return conn, nil
}

// BlobRecord is a BSON-unmarshalling structure for the "blobs" collection in our crawler MongoDBs
type BlobRecord struct {
	Filename string        `bson:"filename"`
	Size     int           `bson:"size"`
	Sha256   string        `bson:"sha256"`
	Job      string        `bson:"job"` // DEPRECATED (old schema)
	Type     string        `bson:"type"`
	PageID   bson.ObjectId `bson:"pageId"`
}

// BlobSetRecord is a BSON-unmarshalling structure for the "blob_set" collection in our crawler MongoDBs
type BlobSetRecord struct {
	Sha256     string        `bson:"sha256"`
	FileID     bson.ObjectId `bson:"file_id"`
	Data       []byte        `bson:"data"`
	Compressed bool          `bson:"z"`
}

func getBlobReaderByOid(db *mgo.Database, blobOid bson.ObjectId) (io.Reader, error) {
	var err error

	blobs := db.C("blobs")

	var blob BlobRecord
	err = blobs.FindId(blobOid).One(&blob)
	if err != nil {
		return nil, err
	}

	return getBlobReaderByHash(db, blob.Sha256, (blob.Type == "vv8logz"))
}

func getBlobReaderByHash(db *mgo.Database, hexSha256 string, forceUnzip bool) (io.Reader, error) {
	blobSet := db.C("blob_set")

	var entry BlobSetRecord
	err := blobSet.Find(bson.M{"sha256": hexSha256}).One(&entry)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if entry.Data == nil {
		// GridFS record
		reader, err = db.GridFS("fs").OpenId(entry.FileID)
		if err != nil {
			return nil, err
		}
	} else {
		// Inline record
		reader = bytes.NewReader(entry.Data)
	}

	// Should it inflate on the fly?
	if entry.Compressed || forceUnzip {
		reader, err = gzip.NewReader(NewClosingReader(reader))
		if err != nil {
			return nil, err
		}
	}

	return NewClosingReader(reader), nil
}

// NewBlobDataReader makes a Reader for slurping a blob out of Mongo (identified by blob SHA2-256 hash)
func NewBlobDataReader(db *mgo.Database, hexSha256 string) (io.Reader, error) {
	return getBlobReaderByHash(db, hexSha256, false)
}

// VV8LogRecord is a BSON-unmarshalling structure for the "vv8logs" collection in our crawler MongoDBs
type VV8LogRecord struct {
	ID         bson.ObjectId   `bson:"_id"`
	RootName   string          `bson:"root_name"`
	LastUpdate time.Time       `bson:"last_update"`
	PageID     bson.ObjectId   `bson:"pageId"`
	BlobIds    []bson.ObjectId `bson:"blobs"`
}

// GetVV8LogRecord returns a vv8logs record by OID
func GetVV8LogRecord(db *mgo.Database, vv8LogOid bson.ObjectId) (*VV8LogRecord, error) {
	var vv8log VV8LogRecord
	err := db.C("vv8logs").FindId(vv8LogOid).One(&vv8log)
	if err != nil {
		return nil, err
	}
	return &vv8log, nil
}

// Reader constructs a log-contents reader for a loaded VV8LogRecord that handles multi-part log blobs
func (vv8log *VV8LogRecord) Reader(db *mgo.Database) (io.Reader, error) {
	if len(vv8log.BlobIds) > 0 {
		// Old-style multi-blob vv8log
		var readers []io.Reader
		for _, blobOid := range vv8log.BlobIds {
			log.Printf("Opening blob %v", blobOid)
			reader, err := getBlobReaderByOid(db, blobOid)
			if err != nil {
				return nil, err
			}
			readers = append(readers, reader)
		}
		return io.MultiReader(readers...), nil
	}

	// New-style single-gzip-GridFile vv8log
	mgoStream, err := db.GridFS("fs").Open(vv8log.RootName)
	if err != nil {
		return nil, err
	}
	zipStream, err := gzip.NewReader(NewClosingReader(mgoStream))
	if err != nil {
		return nil, err
	}
	return NewClosingReader(zipStream), nil
}

const inlineBlobSize = 1024 * 1024

func insertUniqueBlob(db *mgo.Database, data []byte, z bool, preSha256 string) (string, bson.ObjectId, error) {
	var hexSha256 string
	if z {
		if preSha256 == "" {
			return "", "", fmt.Errorf("must provide sha256 for compressed data")
		}
		hexSha256 = preSha256
	} else {
		rawSha256 := sha256.Sum256(data)
		hexSha256 = hex.EncodeToString(rawSha256[:])
	}

	id := bson.NewObjectId()
	doc := bson.M{
		"_id":    id,
		"sha256": hexSha256,
		"z":      z,
	}

	blobSet := db.C("blob_set")
	err := blobSet.Insert(doc)
	if err == nil {
		// We won the race--the prize is inserting the actual data...
		if len(data) < inlineBlobSize {
			// Store these inline
			err = blobSet.UpdateId(id, bson.M{"$set": bson.M{"data": data}})
			if err != nil {
				return "", "", err
			}
		} else {
			// This calls for GridFS...
			file, err := db.GridFS("fs").Create(hexSha256)
			if err != nil {
				return "", "", err
			}
			defer file.Close()

			// write the data; if anything goes wrong,
			// try to delete the blob_set entry on the way out
			n, err := file.Write(data)
			if err != nil {
				blobSet.RemoveId(id)
				return "", "", err
			} else if n < len(data) {
				blobSet.RemoveId(id)
				return "", "", fmt.Errorf("GridFS short write")
			}

			// update the blob_set entry to indicate where the data is
			err = blobSet.UpdateId(id, bson.M{"$set": bson.M{"file_id": file.Id()}})
			if err != nil {
				return "", "", err
			}
		}
		return hexSha256, id, nil
	} else if mgo.IsDup(err) {
		// Look up the original OID
		var doc struct {
			id bson.ObjectId `bson:"_id"`
		}
		err = blobSet.Find(bson.M{"sha256": hexSha256}).One(&doc)
		if err != nil {
			return "", "", err
		}
		return hexSha256, doc.id, nil
	} else {
		// A real (non-dup) error
		return "", "", err
	}
}

// ArchiveBlob handles packaging a block of raw bytes up into crawler MongoDB blob, including deduplication
func ArchiveBlob(db *mgo.Database, name string, data []byte, compressed bool, meta bson.M) (string, bson.ObjectId, error) {
	preSha256, ok := meta["sha256"].(string)
	if !ok {
		preSha256 = ""
	}
	// Get the data stashed, then worry about the metadata
	sha256, _, err := insertUniqueBlob(db, data, compressed, preSha256)
	if err != nil {
		return "", "", err
	}

	if meta == nil {
		meta = bson.M{}
	}
	bid := bson.NewObjectId()
	meta["_id"] = bid
	meta["sha256"] = sha256
	meta["filename"] = name
	meta["size"] = len(data)
	meta["z"] = compressed
	blobs := db.C("blobs")
	err = blobs.Insert(meta)
	if err != nil {
		return "", "", err
	}

	return sha256, bid, nil
}

func gzipInMemory(data []byte) ([]byte, error) {
	var scratch bytes.Buffer
	gz := gzip.NewWriter(&scratch)
	_, err := gz.Write(data)
	if err != nil {
		return nil, err
	}
	err = gz.Close()
	if err != nil {
		return nil, err
	}
	return scratch.Bytes(), nil
}

// CompressBlob compresses the data before archiving the blob (in memory)
func CompressBlob(db *mgo.Database, name string, data []byte, meta bson.M) (string, bson.ObjectId, error) {
	// Compress the data in-memory
	zdata, err := gzipInMemory(data)
	if err != nil {
		return "", "", err
	}

	if meta == nil {
		meta = bson.M{}
	}
	rawSha256 := sha256.Sum256(data)
	meta["sha256"] = hex.EncodeToString(rawSha256[:])
	meta["orig_size"] = len(data)

	return ArchiveBlob(db, name, zdata, true, meta)
}

// MarkVV8LogComplete adds a metadata record to the original "vv8logs" record
func MarkVV8LogComplete(db *mgo.Database, logID bson.ObjectId, formats ...string) error {
	now := bson.Now()
	fields := bson.M{}
	for _, f := range formats {
		fields["completed."+f] = now
	}
	update := bson.M{"$set": fields}
	return db.C("vv8logs").UpdateId(logID, update)
}

// GetRootDomain looks up "jobs[job=ln.Job].alexa_domain" (old style schema) or "pages[_id=ln.PageId].context.rootDomain" (new style schema)
func GetRootDomain(db *mgo.Database, ln *LogInfo) (string, error) {
	if ln.PageID.Valid() {
		page := bson.M{}
		err := db.C("pages").FindId(ln.PageID).One(page)
		if err != nil {
			return "", err
		}
		rawContext, ok := page["context"]
		if !ok {
			return "", fmt.Errorf("page %s has no 'context'", ln.PageID.Hex())
		}
		context, ok := rawContext.(bson.M)
		if !ok {
			return "", fmt.Errorf("page %s's 'context' is not an object", ln.PageID.Hex())
		}
		rootDomain, ok := context["rootDomain"]
		if !ok {
			return "", fmt.Errorf("page %s's 'context' has no 'rootDomain'", ln.PageID.Hex())
		}
		return rootDomain.(string), nil
	} else if ln.Job != "" {
		job := bson.M{}
		err := db.C("jobs").Find(bson.M{"job": ln.Job}).One(job)
		if err != nil {
			return "", err
		}
		domainName, ok := job["alexa_domain"]
		if !ok {
			return "", fmt.Errorf("job %s has no 'alexa_domain' attribute", ln.Job)
		}
		return domainName.(string), nil
	} else {
		log.Printf("warning: no Job/PageID info for log %s\n", ln.RootName)
		return "", nil
	}
}
