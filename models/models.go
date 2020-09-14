package models

import (
	"go.etcd.io/bbolt"
	"log"
	"time"

	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/pop/v5"
)

// DB is a connection to your database to be used
// throughout your application.
var DB *pop.Connection

// BDB is a transaction in the simple BBolt database
// for fulfilling database needs when a full blown
// SQL server is not required
var BDB *bbolt.DB

func init() {
	// SQL initialization
	var err error
	env := envy.Get("GO_ENV", "development")
	DB, err = pop.Connect(env)
	if err != nil {
		log.Fatal(err)
	}
	pop.Debug = env == "development"

	// Begin BBolt initialization
	BDB, err = bbolt.Open("tmp/bbolt.db", 0600, &bbolt.Options{Timeout: 4 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	// any new bucket names must be created here
	_ = BDB.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("pyUploads"))
		must(err)
		_, err = tx.CreateBucketIfNotExists([]byte("safeUsers"))
		must(err)
		return nil
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
