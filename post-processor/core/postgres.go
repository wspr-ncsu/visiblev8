package core

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/lib/pq"
	pubsuf "golang.org/x/net/publicsuffix"
	"gopkg.in/mgo.v2/bson"
)

// NullableRune returns either string(<val>) (if not 0) or nil
func NullableRune(val rune) interface{} {
	var nullable interface{}
	if val != 0 {
		nullable = string(val)
	}
	return nullable
}

// NullableString returns either <val> (if not "") or nil
func NullableString(val string) interface{} {
	var nullable interface{}
	if val != "" {
		nullable = val
	}
	return nullable
}

// NullableMongoOID returns either <val> (if .Valid() == true) or nil
func NullableMongoOID(val bson.ObjectId) interface{} {
	var nullable interface{}
	if val.Valid() {
		nullable = val
	}
	return nullable
}

// NullableBytes returns either <val> (if not length 0) or nil
func NullableBytes(val []byte) interface{} {
	var nullable interface{}
	if len(val) == 0 {
		nullable = val
	}
	return nullable
}

// NullableInt returns either <val> (if not 0) or nil
func NullableInt(val int) interface{} {
	var nullable interface{}
	if val != 0 {
		nullable = val
	}
	return nullable
}

// NullableTimestamp returns either <val> (if not .IsZero()) or nil
func NullableTimestamp(val time.Time) interface{} {
	var nullable interface{}
	if !val.IsZero() {
		nullable = val
	}
	return nullable
}

// CreateImportTable creates a temp table copying the schema of a given prototype table
func CreateImportTable(sqlDb *sql.DB, likeTable, importTableName string) error {
	_, err := sqlDb.Exec(fmt.Sprintf(`CREATE TEMP TABLE "%s" (LIKE "%s" INCLUDING DEFAULTS INCLUDING INDEXES);`, importTableName, likeTable))
	if err != nil {
		return err
	}
	return nil
}

// BulkFieldGenerator is a callback-iterator pattern for streaming row values into a bulk-import table/transaction
type BulkFieldGenerator func() ([]interface{}, error)

// BulkInsertRows performs a bulk-insert transaction, streaming callback-provided data into a temp import table
func BulkInsertRows(sqlDb *sql.DB, functionName, tableName string, fieldNames []string, generator BulkFieldGenerator) (int64, error) {
	var rowCount int64

	txn, err := sqlDb.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if txn != nil {
			log.Printf("%s: defer-triggered txn.Rollback()...", functionName)
			if rollbackErr := txn.Rollback(); rollbackErr != nil {
				log.Printf("%s: txn.Rollback() failed: %v\n", functionName, rollbackErr)
			}
		}
	}()

	stmt, err := txn.Prepare(pq.CopyIn(tableName, fieldNames...))
	if err != nil {
		log.Printf("%s: txn.Prepare(...) failed: %v\n", functionName, err)
		return 0, err
	}
	defer func() {
		if stmt != nil {
			log.Printf("%s: defer-triggered stmt.Close()...", functionName)
			if err := stmt.Close(); err != nil {
				log.Printf("%s: stmt.Close() failed: %v\n", functionName, err)
			}
		}
	}()

	lastProgressReport := time.Now()
	for {
		values, err := generator()
		if err != nil { // error/abort (rollback)
			log.Printf("%s: generator(...) failed: %v\n", functionName, err)
			return 0, err
		} else if values == nil { // end-of-stream (commit)
			break
		} else { // data (insert)
			_, err = stmt.Exec(values...)
			if err != nil {
				log.Printf("%s: stmt.Exec(...) failed: %v\n", functionName, err)
				return 0, err
			}
			rowCount++
			if time.Now().Sub(lastProgressReport) >= (time.Second * 5) {
				log.Printf("%s: processed %d records so far...\n", functionName, rowCount)
				lastProgressReport = time.Now()
			}
		}
	}
	log.Printf("%s: done processing after %d records\n", functionName, rowCount)

	_, err = stmt.Exec()
	if err != nil {
		log.Printf("%s: final stmt.Exec() failed: %v\n", functionName, err)
		return 0, err
	}
	err = stmt.Close()
	stmt = nil // nothing to close now
	if err != nil {
		log.Printf("%s: stmt.Close() failed: %v\n", functionName, err)
		return 0, err
	}
	err = txn.Commit()
	txn = nil // nothing to rollback now
	if err != nil {
		return 0, err
	}

	return rowCount, nil
}

// SHA2Block is storage for a SHA256 digest
type SHA2Block [sha256.Size]byte

// bakedURL is a URL waiting for insertion into the `urls` table
type bakedURL struct {
	Sha256   SHA2Block
	Full     string
	Scheme   string
	Hostname string
	Port     string
	Path     string
	Query    string
	Etld1    string
	Stemmed  string
}

// urlsFields holds the in-order list of field names used for bulk-inserting crawl records into a temp-clone of `urls_import_schema`
var urlImportFields = [...]string{
	"sha256",
	"url_full",
	"url_scheme",
	"url_hostname",
	"url_port",
	"url_path",
	"url_query",
	"url_etld1",
	"url_stemmed",
}

// URLBakery keeps a stash of cooked URLs pending insertion
type URLBakery struct {
	stash map[string]*bakedURL
}

// NewURLBakery instantiates a fresh, empty URLBakery
func NewURLBakery() *URLBakery {
	return &URLBakery{
		stash: make(map[string]*bakedURL),
	}
}

// URLToHash takes a raw URL string and returns its SHA256 hash (after stashing it if it is new/unseen)
func (ub *URLBakery) URLToHash(rawurl string) SHA2Block {
	curl, ok := ub.stash[rawurl]
	if !ok {
		curl = &bakedURL{
			Sha256: sha256.Sum256([]byte(rawurl)),
			Full:   rawurl,
		}

		purl, err := url.Parse(rawurl)
		if err != nil {
			log.Printf("urlBakery.toHash: error (%v) parsing '%s'; no fields available\n", err, rawurl)
		} else {
			curl.Scheme = purl.Scheme
			curl.Hostname = purl.Hostname()
			curl.Port = purl.Port()
			curl.Path = purl.EscapedPath()
			curl.Query = purl.RawQuery

			etld1, err := pubsuf.EffectiveTLDPlusOne(purl.Hostname())
			if err != nil {
				curl.Etld1 = purl.Hostname()
			} else {
				curl.Etld1 = etld1
			}
			curl.Stemmed = curl.Etld1 + curl.Path
		}
		ub.stash[rawurl] = curl
	}
	return curl.Sha256
}

// InsertBakedURLs performs a de-duping bulk insert of cooked URL records into PG's `urls` table
func (ub *URLBakery) InsertBakedURLs(sqlDb *sql.DB) error {
	if len(ub.stash) == 0 {
		log.Println("urlBakery.insertBakedURLs: no baked URLs in the oven; nothing to do!")
		return nil
	}

	log.Println("urlBakery.insertBakedURLs: creating temp table 'import_urls'...")
	err := CreateImportTable(sqlDb, "urls_import_schema", "import_urls")
	if err != nil {
		log.Printf("urlBakery.insertBakedURLs: createImportTable(...) failed: %v\n", err)
		return err
	}
	defer func() {
		log.Printf("urlBakery.insertBakedURLs: dropping temp import table...\n")
		_, err := sqlDb.Exec(`DROP TABLE import_urls;`)
		if err != nil {
			log.Printf("urlBakery.insertBakedURLs: error (%v) dropping `import_urls` temp table\n", err)
		}
	}()

	urlChan := make(chan *bakedURL)
	go func() {
		for _, curl := range ub.stash {
			urlChan <- curl
		}
		close(urlChan)
	}()

	log.Println("urlBakery.insertBakedURLs: bulk-inserting...")
	importRows, err := BulkInsertRows(sqlDb, "urlBakery.insertBakedURLs", "import_urls", urlImportFields[:], func() ([]interface{}, error) {
		curl, ok := <-urlChan
		if !ok {
			log.Printf("urlBakery.insertBakedURLs: iteration complete, committing transation...\n")
			return nil, nil // signal end-of-stream
		}

		values := []interface{}{
			curl.Sha256[:],
			curl.Full,
			NullableString(curl.Scheme),
			NullableString(curl.Hostname),
			NullableString(curl.Port),
			NullableString(curl.Path),
			NullableString(curl.Query),
			NullableString(curl.Etld1),
			NullableString(curl.Stemmed),
		}
		return values, nil
	})
	if err != nil {
		return err
	}

	log.Println("urlBakery.insertBakedURLs: copy-inserting from temp table...")
	result, err := sqlDb.Exec(`
INSERT INTO urls (
		sha256, url_full, url_scheme, url_hostname, url_port,
		url_path, url_query, url_etld1, url_stemmed)
	SELECT
		iu.sha256, iu.url_full, iu.url_scheme, iu.url_hostname, iu.url_port,
		iu.url_path, iu.url_query, iu.url_etld1, iu.url_stemmed
	FROM import_urls AS iu
ON CONFLICT DO NOTHING;
`)
	if err != nil {
		return err
	}
	insertRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	log.Printf("urlBakery.insertBakedURLs: inserted %d (out of %d) import rows\n", insertRows, importRows)

	return nil
}
