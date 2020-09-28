package models

// This is not an SQL models file. This contains
// the middleware for a BBolt database, which is
// a simple key/value store when a full blown
// SQL server is not needed.
//
// Patricio Whittingslow 2020

import (
	"errors"
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/markbates/errx"
	"go.etcd.io/bbolt"
)

var errNonSuccess = errors.New("non success status code")

// BBoltTransaction is a piece of Buffalo middleware that wraps each
// request in a BBoltDB transaction. The transaction will automatically get
// committed if there's no errors and the response status code is a
// 2xx or 3xx, otherwise it'll be rolled back. It will also add a
// field to the log, "bdb", that shows the total duration spent during
// the request writing + spilling + re-balancing.
// This function is nearly an identical copy of pop's Transaction()
// just adapted to BBolt. Databases should be defined/initialized in models/models.go
// One important thing to not is that a writable transaction locks the database
// to other writable transaction, which means only one transaction will be processed
// if there are multiple goroutines waiting to read/write as only ONE read/write tx
// can exist at a time. This may also not scale well if there
// are many random write operations happening in a single transaction.
// BBolt is more adept at small operations.
// see https://github.com/boltdb/bolt#caveats--limitations for more information.
func BBoltTransaction(db *bbolt.DB) buffalo.MiddlewareFunc {
	return func(h buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			// wrap all requests in a transaction and set the length
			// of time doing things in the db to the log.
			// ANY error returned by the tx function will cause the
			// tx to be rolled back
			bTx, err := db.Begin(true)
			if err != nil {
				return fmt.Errorf("in begin bbolt Tx: %s", err)
			}
			// Wrap transaction in closure. this simplifies error handling
			// all we gotta do is return the error and do checking outside
			couldBeDBorYourErr := func() (txError error) {
				// log database usage statistics to context
				defer func() {
					if txError != nil {
						err = bTx.Rollback()
					} else {
						err = bTx.Commit()
					}
					if err != nil { // if BBoltDB fails we replace error
						txError = errx.Wrap(err, "BBoltDB committing/rolling back fail")
					}
					stats := bTx.Stats()
					elapsed := stats.WriteTime + stats.SpillTime + stats.RebalanceTime
					c.LogField("bdb", elapsed)
				}()

				// add transaction to context
				c.Set("btx", bTx)

				// call the next handler; if it errors stop and return the error
				if txError = h(c); txError != nil {
					return
				}
				// check the response status code. if the code is NOT 200..399
				// then it is considered "NOT SUCCESSFUL" and an error will be returned
				if res, ok := c.Response().(*buffalo.Response); ok {
					if res.Status < 200 || res.Status >= 400 {
						txError = errNonSuccess
					}
				}
				// return error if present
				return
			}()
			// couldBeDBorYourErr could be one of possible values:
			// * nil - everything went well, if so, return
			// * yourError - an error returned from your application, middleware, etc...
			// * a database error - this is returned if there were problems committing the transaction
			// * a errNonSuccess - this is returned if the response status code is not between 200..399
			if couldBeDBorYourErr != nil && errx.Unwrap(couldBeDBorYourErr) != errNonSuccess {
				c.Logger().Errorf("couldBeDBorYourErr not nil and not statusErr: %s", couldBeDBorYourErr)
				return couldBeDBorYourErr
			}

			// as far was we can tell everything went well
			return nil
		}
	}
}
