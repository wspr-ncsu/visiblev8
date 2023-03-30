package core

// utilities for I/O to Mongo collections used by vv8/carburetor:
// * get-blob, store-blob
// * get-job-domain, update-job-status
// * stream/decompress multi-part log blobs through a Reader interface

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getDialURL constructs (from environment tuning variables) a MongoDB connection URL
func getDialURL() string {
	host := GetEnvDefault("MONGODB_HOST", "localhost")
	port := GetEnvDefault("MONGODB_PORT", "27017")
	db := GetEnvDefault("MONGODB_AUTHDB", "admin")
	user := GetEnvDefault("MONGODB_USER", "")
	pass := GetEnvDefault("MONGODB_PASS", "")
	return fmt.Sprintf("mongodb://%s:%s@%s:%s/%s", user, pass, host, port, db)
}

// MongoConnection bundles together essential state and stats for a live MongoDB connection
type MongoConnection struct {
	URL    string        // connection URL dialed ("mongodb://$host:$port/$db")
	User   string        // username authenticated as (if applicable; "" otherwise)
	Client *mongo.Client // Active (and possibly authenticated) session to Mongo
}

func (mc MongoConnection) String() string {
	var active, auth string
	if mc.Client != nil {
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
	// Never cancel this context, since we want to keep the connection open
	ctx, cancel := context.WithTimeout(context.Background(), 3600*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(conn.URL))
	if err != nil {
		return MongoConnection{}, err
	}
	conn.Client = client
	return conn, nil
}
