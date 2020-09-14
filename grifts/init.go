package grifts

import (
	"github.com/gobuffalo/buffalo"
	"github.com/soypat/curso/actions"
)

func init() {
	buffalo.Grifts(actions.App())
}
