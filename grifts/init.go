package grifts

import (
	"github.com/gobuffalo/buffalo"
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/actions"
)

func init() {
	buffalo.Grifts(actions.App())
}
