package actions

import (
	"strings"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
)

// EvaluationIndex default implementation.
func EvaluationIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	evals := &models.Evaluations{}
	if err := tx.Order("created_at ASC").All(evals); err != nil {
		return c.Error(404, err)
	}
	// sort.Sort(evals)
	c.Set("evaluations", evals)
	c.Logger().Debugf("Finishing EvaluationIndex with c.Data():%v", c.Data())
	return c.Render(200, r.HTML("curso/eval-index.plush.html"))
}

// CursoEvaluationCreateGet renders evaluation creation page
func CursoEvaluationCreateGet(c buffalo.Context) error {
	c.Set("evaluation", models.Evaluation{})
	return c.Render(200, r.HTML("curso/eval-create.plush.html"))
}

// CursoEvaluationCreatePost handles creation of evaluation
func CursoEvaluationCreatePost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	eval := &models.Evaluation{}
	if err := c.Bind(eval); err != nil {
		return errors.WithStack(err)
	}
	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(eval)
	if err != nil {
		return errors.WithStack(err)
	}
	c.Set("evaluation", eval)
	if verrs.HasAny() {
		c.Flash().Add("danger", T.Translate(c, "curso-python-evaluation-add-fail"))
		return c.Render(422, r.HTML("topics/create.plush.html"))
	}
	u := c.Value("current_user").(*models.User)
	c.Logger().Infof("evaluation create %s, by %s", eval.Title, u.Email)
	c.Flash().Add("success", T.Translate(c, "curso-python-evaluation-add-success"))
	return c.Render(200, r.HTML("curso/eval-get.plush.html"))
}

// CursoEvaluationEditGet handles the rendering of the evaluation edit page
func CursoEvaluationEditGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	eval := &models.Evaluation{}
	eid := c.Param("evalid")
	q := tx.Where("id = ?", eid)
	if err := q.First(eval); err != nil {
		return c.Error(404, err)
	}
	c.Set("evaluation", eval)
	return c.Render(200, r.HTML("curso/eval-create.plush.html"))
}

// CursoEvaluationEditPost handles the editing of an already existing evaluation
func CursoEvaluationEditPost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	eid := c.Param("evalid")

	eval := &models.Evaluation{}
	q := tx.Where("id = ?", eid)
	if err := q.First(eval); err != nil {
		return errors.WithStack(err)
	}
	uid := eval.ID
	if err := c.Bind(eval); err != nil {
		return errors.WithStack(err)
	}
	eval.ID = uid
	// Validate the data from the html form
	if err := tx.Update(eval); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "edit-success"))
	return c.Redirect(302, "evaluationGetPath()", render.Data{"evalid": eval.ID})
}

// CursoEvaluationDelete handles deletion event of evaluation
func CursoEvaluationDelete(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	eid := c.Param("evalid")
	eval := &models.Evaluation{}
	q := tx.Where("id = ?", eid)
	if err := q.First(eval); err != nil {
		return errors.WithStack(err)
	}
	eval.Deleted = true
	if err := tx.Update(eval); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "delete-success"))
	return c.Redirect(302, "evaluationPath()")
}

// CursoEvaluationGet handles rendering of an evaluation
func CursoEvaluationGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	eval := &models.Evaluation{}
	eid := c.Param("evalid")
	q := tx.Where("id = ?", eid)
	if err := q.First(eval); err != nil {
		return c.Error(404, err)
	}
	c.Set("evaluation", eval)
	return c.Render(200, r.HTML("curso/eval-get.plush.html"))
}

func PassedEvaluationHandler(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		tx := c.Value("tx").(*pop.Connection)
		evals := &models.Evaluations{}
		if err := tx.Where("deleted = ?", false).All(evals); err != nil {
			return c.Error(500, err)
		}
		user := c.Value("current_user").(*models.User)
		c.Set("evaluations", evals)
		for _, e := range *evals {
			if strings.Contains(strings.ToLower(normalize(e.Title)), "desafio final") && !user.Subscribed(e.ID) {
				c.Flash().Add("warning", T.Translate(c, "evaluation-pass-required", e))
				c.Logger().Infof("user %s not passed. bounce back", user.Email)
				return c.Redirect(302, c.Request().Referer())
			}
		}
		c.Logger().Infof("user %s passed. allowing to continue", user.Email)
		return next(c)
	}
}

// evaluationsPassed checks if user passed all final evaluations
// does not check deleted evaluations but does check hidden evaluations
func evaluationsPassed(c buffalo.Context, user *models.User) (bool, error) {
	tx := c.Value("tx").(*pop.Connection)
	evals := &models.Evaluations{}
	if err := tx.Where("deleted = ?", false).All(evals); err != nil {
		return false, err
	}
	for _, e := range *evals {
		if strings.Contains(strings.ToLower(normalize(e.Title)), "desafio final") && !user.Subscribed(e.ID) {
			e.Title = deleteXMLTags(e.Title)
			c.Flash().Add("warning", T.Translate(c, "evaluation-pass-required", e))
			return false, nil
		}
	}
	return true, nil
}
func deleteXMLTags(s string) string {
	var b strings.Builder
	done := false
	idx := 0
	for !done { // idx0 y idx1 en coordenadas relativas a idx
		idx0 := strings.Index(s[idx:], "<")
		idx1 := strings.Index(s[idx:], ">")
		if idx0 < 0 || idx1 < 0 {
			if idx == 0 {
				return s
			}
			b.WriteString(s[idx:])
			break
		}
		if idx0 > idx1 {
			idx = idx1
			continue
		}
		ess := s[idx : idx+idx0]
		b.WriteString(ess)
		idx += idx1 + 1
	}
	return b.String()
}
