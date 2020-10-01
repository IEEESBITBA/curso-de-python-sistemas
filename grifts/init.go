package grifts

import (
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/actions"
	"github.com/gobuffalo/buffalo"
)

func init() {
	buffalo.Grifts(actions.App())
}
