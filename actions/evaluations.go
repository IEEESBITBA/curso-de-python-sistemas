package actions

import (
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
)

// CategoriesIndex default implementation.
func EvaluationIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	evals := &models.Evaluations{}
	if err := tx.All(evals); err != nil {
		return c.Error(404, err)
	}
	c.Set("evaluations", evals)
	return c.Render(200, r.HTML("curso/eval-index.plush.html"))
}

func CursoEvaluationCreateGet(c buffalo.Context) error {
	e := models.Evaluation{}
	c.Set("evaluation",e)
	return c.Render(200, r.HTML("curso/eval-create.plush.html"))
}

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
