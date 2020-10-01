package grifts

import (
	"github.com/markbates/grift/grift"
)

var _ = grift.Namespace("db", func() {

	_ = grift.Desc("seed", "Seeds a database")
	_ = grift.Add("seed", func(c *grift.Context) error {
		// Add DB seeding stuff here
		return nil
	})

})
