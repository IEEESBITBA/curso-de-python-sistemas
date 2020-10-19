package actions

import (
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
)

// SubmissionsIndex default implementation.
func SubmissionsIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	subs := &models.Submissions{}
	if err := tx.Where("is_template = ?", true).Order("created_at ASC").All(subs); err != nil {
		return c.Error(500, err)
	}
	// sort.Sort(evals)
	c.Set("submissions", subs)
	c.Logger().Debugf("Finishing submissionIndex with c.Data():%v", c.Data())
	return c.Render(200, r.HTML("submissions/index.plush.html"))
}

func SubmissionCreateGet(c buffalo.Context) error {
	c.Set("submission", models.Submission{})
	return c.Render(200, r.HTML("submissions/create.plush.html"))
}
