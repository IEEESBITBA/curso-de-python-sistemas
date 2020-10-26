package actions

import (
	"fmt"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/tags/v3"
	"github.com/gobuffalo/validate/v3"
	yaml "github.com/goccy/go-yaml"
	"github.com/gofrs/uuid"
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
	return c.Render(200, r.HTML("submissions/index.plush.html"))
}

func SubmissionGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	sub := new(models.Submission)
	q := tx.Where("id = ?", c.Param("sid"))
	if err := q.First(sub); err != nil {
		return c.Error(404, err)
	}

	R := unmarshalYaml(c, sub)
	c.Set("form_data", R)
	c.Set("submission", sub)
	return c.Render(200, r.HTML("submissions/get.plush.html"))
}

func SubmissionCreateGet(c buffalo.Context) error {
	sub := new(models.Submission)
	if c.Param("sid") != "" {
		tx := c.Value("tx").(*pop.Connection)
		if err := tx.Where("id = ?", c.Param("sid")).First(sub); err != nil {
			return c.Error(500, err)
		}
	}

	c.Set("submission", sub)
	return c.Render(200, r.HTML("submissions/create.plush.html"))
}

// SubmissionCreatePost handles creation of submission
func SubmissionCreatePost(c buffalo.Context) error {
	var err error
	var editing bool
	var verrs *validate.Errors
	tx := c.Value("tx").(*pop.Connection)
	sub := &models.Submission{}
	if err = c.Bind(sub); err != nil {
		return c.Error(500, err)
	}
	user := c.Value("current_user").(*models.User)
	forum := c.Value("forum").(*models.Forum)
	sub.IsTemplate = true
	sub.ForumID, sub.UserID = forum.ID, user.ID
	c.Set("submission", sub)
	if err = validateSubmissionForm(unmarshalYaml(c, sub)); err != nil {
		c.Flash().Add("warning", T.Translate(c, "submission-schemas-validation-fail")+err.Error())
		return c.Render(200, r.HTML("submissions/create.plush.html"))
	}
	if c.Param("sid") != "" {
		editing = true
		sub.ID, _ = uuid.FromString(c.Param("sid"))
		verrs, err = tx.ValidateAndUpdate(sub)
	} else {
		verrs, err = tx.ValidateAndCreate(sub)
	}

	// Validate the data from the html form
	if err != nil {
		return c.Error(500, err)
	}

	if verrs.HasAny() {
		c.Logger().Errorf("Error adding submission: %v", verrs.Errors)
		c.Flash().Add("danger", T.Translate(c, "submission-add-fail"))
		return c.Render(500, r.HTML("submissions/create.plush.html"))
	}
	u := c.Value("current_user").(*models.User)

	if editing {
		c.Flash().Add("success", T.Translate(c, "submission-edit-success"))
		c.Logger().Infof("submission edit %s, by %s", sub.Title, u.Email)
	} else {
		c.Flash().Add("success", T.Translate(c, "submission-add-success"))
		c.Logger().Infof("submission create %s, by %s", sub.Title, u.Email)
	}
	return c.Redirect(302, "subGetPath()", render.Data{"forum_title": forum.Title, "sid": sub.ID.String()})
}

func SubmissionDelete(c buffalo.Context) error {
	return c.Error(500, fmt.Errorf("Not implemented!"))
}

func SubmissionSubmitPost(c buffalo.Context) error {
	return c.Error(500, fmt.Errorf("Not implemented!"))
}

// SUBMISSIONS RENDERING

func unmarshalYaml(c buffalo.Context, s *models.Submission) *[]tags.Options {
	r := new([]tags.Options)
	err := yaml.Unmarshal([]byte(s.Schemas.String), r)
	if err != nil {
		c.Logger().Errorf("bad schema: %s", s.Schemas.String)
	}
	return r
}

// func prepareSubmissionInput(r *[]tags.Options) *[]tags.Options {
// 	for i := range *r {
// 		for attr, val := range (*r)[i] {
// 			s, ok := val.(string)
// 			if !ok {
// 				continue
// 			}
// 			switch {
// 			case attr == "type" && s == "text":
// 				(*r)[i]["rows"] = 1
// 			}
// 		}
// 	}
// 	return r
// }

func validateSubmissionForm(r *[]tags.Options) error {
	type void struct{}
	names := make(map[string]void)
	var null void
	for _, input := range *r {
		if err := validateSubmissionInput(input); err != nil {
			return err
		}
		name := input["name"].(string)
		_, ok := names[name]
		if ok {
			return fmt.Errorf("names should be unique. Offending name: %q is repeated for %q", name, input["label"].(string))
		}
		names[name] = null
	}
	return nil
}
func validateSubmissionInput(r tags.Options) error {
	theType, okType := r["type"]
	theLabel, okLabel := r["label"]
	theName, okName := r["name"]
	if !okType || !okLabel || !okName {
		return fmt.Errorf(
			"Did not find some required attribute(s) in schema.\n"+
				"Passed: {type:%t, label:%t, name:%t}", okType, okLabel, okName)
	}
	castType, okType := theType.(string)
	_, okLabel = theLabel.(string)
	_, okName = theName.(string)
	if !okType || !okLabel || !okName {
		return fmt.Errorf(
			"Some attribute(s) is not a string. Check YAML spec on types.\n"+
				"Passed: {type:%t, label:%t, name:%t}", okType, okLabel, okName)
	}
	switch castType {
	case "dropdown":
		_, okOpts := r["options"]
		if !okOpts {
			return fmt.Errorf(
				"Did not find some required attribute(s) in dropdown schema."+
					"options:%t", okOpts)
		}
	}
	return nil
}
