package actions

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

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
	user := c.Value("current_user").(*models.User)
	R := unmarshalYaml(c, sub)
	c.Set("form_data", R)
	c.Set("submission", sub)
	c.Set("form_error", userSubmissionGreenlight(c, user, sub))
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
	tx := c.Value("tx").(*pop.Connection)
	template := new(models.Submission)
	if err := tx.Where("id = ?", c.Param("sid")).First(template); err != nil {
		return c.Error(500, err)
	}
	template.Deleted = true
	if err := tx.UpdateColumns(template, "deleted"); err != nil {
		return c.Error(500, err)
	}
	c.Flash().Add("success", "Delete submission success")
	return c.Redirect(302, "subIndexPath()", render.Data{"forum_title": c.Param("forum_title"), "sid": c.Param("sid")})
}

func SubmissionSubmitPost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	template := new(models.Submission)

	if err := tx.Where("id = ?", c.Param("sid")).First(template); err != nil {
		return c.Error(500, err)
	}
	ctxData := render.Data{"sid": c.Param("sid"), "forum_title": c.Param("forum_title")}
	inputs := unmarshalYaml(c, template)
	zipbuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipbuf)
	fileCount := 0
	user := c.Value("current_user").(*models.User)
	// prechecks for user submission
	if err := userSubmissionGreenlight(c, user, template); err != nil {
		c.Logger().Infof("user %s not greenlit or error:%s", user.Email, err)
		return c.Redirect(302, c.Request().Referer())
	}
	for _, input := range *inputs {
		if input["type"].(string) == "file" {
			name := input["name"].(string)
			maxSize := int64(input["max_size"].(uint64)) * 1e6
			in, k, err := c.Request().FormFile(name)
			if err != nil {
				if req, ok := input["required"]; ok && req.(bool) || !ok { // if not required, skip it
					continue
				}
				return c.Error(500, err)
			}
			defer in.Close()
			if k.Size > maxSize {
				c.Flash().Add("warning", T.Translate(c, "submission-file-too-big", input))
				return c.Redirect(302, "subGetPath()", ctxData)
			}
			fileCount++

			var folder string
			if f, ok := input["folder"]; ok {
				folder = f.(string) + "/"
			}
			zipFile, err := zipWriter.Create(folder + name + "#" + k.Filename)
			if err != nil {
				c.Flash().Add("danger", T.Translate(c, "submission-file-submit-fail", input))
				return c.Redirect(302, "subGetPath()", ctxData)
			}
			if _, err = io.Copy(zipFile, in); err != nil {
				c.Flash().Add("danger", T.Translate(c, "submission-file-submit-fail", input))
				return c.Redirect(302, "subGetPath()", ctxData)
			}
			c.Request().Form.Set(name, k.Filename)
			in.Close()
		}
	}
	if err := zipWriter.Close(); err != nil {
		c.Flash().Add("danger", T.Translate(c, "submission-file-submit-error"))
		return c.Redirect(302, "subGetPath()", ctxData)
	}
	c.Request().Form.Add("date", time.Now().Format("15:04:05 (MST) 02/01/06"))
	b, err := yaml.Marshal(c.Request().Form)
	if err != nil {
		c.Logger().Errorf("marshal yaml error:%s", err)
		return c.Error(500, err)
	}

	sub := template.Template(user)
	sub.Response.String, sub.Response.Valid = string(b), true
	if fileCount > 0 {
		sub.Zip.ByteSlice, sub.Zip.Valid = zipbuf.Bytes(), true
		sub.HasAttachment = true
	}
	if verrs, err := tx.ValidateAndCreate(sub); err != nil {
		c.Logger().Errorf("Got validation errors for %s submit on %s:%v", user.Email, template.Title.String, verrs)
		return c.Error(500, err)
	}
	user.AddSubscription(template.ID)
	_ = tx.UpdateColumns(user, "subscriptions")
	c.Flash().Add("success", T.Translate(c, "submission-response-recieved"))
	return c.Redirect(302, "subIndexPath()", render.Data{"forum_title": c.Param("forum_title")})
}

func SubmissionDownloadAllResponses(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	subs := new(models.Submissions)
	tstart := time.Now()
	template := new(models.Submission)
	if err := tx.Where("id = ?", c.Param("sid")).First(template); err != nil {
		return c.Error(500, err)
	}
	if err := tx.Where("submission_id = ?", c.Param("sid")).Where("is_template = ?", false).All(subs); err != nil {
		c.Logger().Errorf("error getting all non template submissions for %s", c.Param("sid"))
		return c.Error(500, err)
	}
	c.Logger().Infof("loaded all responses in %s", time.Since(tstart))
	outBuffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(outBuffer)
	defer zipWriter.Close()
	var ymlResponses strings.Builder
	ymlResponses.Grow(len(*subs) * (len((*subs)[0].Response.String) + 5))
	ymlResponses.WriteString(
		fmt.Sprintf("# Responses for %s- Use python's yaml.load_all() function to read.\n# First entry is form template!\n---\n%s",
			template.Title.String, template.Schemas.String))
	var totalWritten int64
	for _, sub := range *subs {
		ymlResponses.WriteString(fmt.Sprintf("\n---\nsub_db_id: %q\n", sub.ID))
		ymlResponses.WriteString(sub.Response.String)
		if !sub.HasAttachment {
			continue
		}
		r, err := zip.NewReader(bytes.NewReader(sub.Zip.ByteSlice), int64(len(sub.Zip.ByteSlice)))
		if err != nil {
			return c.Error(500, err)
		}
		for _, f := range r.File {
			var n int64
			freader, err := f.Open()
			if err != nil {
				return c.Error(500, err)
			}
			_, fname := filepath.Split(f.Name)
			w, err := zipWriter.Create(filepath.Join(sub.ID.String(), fname))
			if err != nil {
				return c.Error(500, err)
			}
			if n, err = io.Copy(w, freader); err != nil {
				return c.Error(500, err)
			}
			freader.Close()
			totalWritten += n
			c.Logger().Debugf("file %s found. %d written", f.Name, n)
		}
	}
	w, err := zipWriter.Create("manifest.yml")
	if err != nil {
		return c.Error(500, err)
	}
	n, err := w.Write([]byte(ymlResponses.String()))
	if err != nil {
		return c.Error(500, err)
	}
	zipWriter.Close()
	totalWritten += int64(n)
	c.Logger().Infof("finished creating responses zip, %0.3fMB written in %s", float64(totalWritten)/1e6, time.Since(tstart))
	return c.Render(200, r.Download(c,
		fmt.Sprintf("%s-responses.zip", template.ID.String()[0:8]),
		outBuffer))
}

func SubmissionResponseIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	page, perPage := setPagination(c.Params(), 20)
	q := tx.Where("submission_id = ?", c.Param("sid")).Where("is_template = ?", false).Paginate(page, perPage)
	subs := new(models.Submissions)
	if err := q.All(subs); err != nil {
		return c.Error(500, err)
	}
	for i := range *subs {
		if (*subs)[i].Anonymous {
			continue
		}
		user := new(models.User)
		if err := tx.Where("id = ?", (*subs)[i].UserID).First(user); err == nil {
			(*subs)[i].User = user
		} else {
			(*subs)[i].User = new(models.User)
		}
	}
	c.Set("template_id", c.Param("sid"))
	c.Set("pagination", q.Paginator)
	c.Set("submissions", subs)
	return c.Render(200, r.HTML("submissions/sub-index.plush.html"))
}

func SubmissionResponseZipDownload(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	sub := new(models.Submission)
	if err := tx.Where("id = ?", c.Param("sid")).First(sub); err != nil {
		return c.Error(500, err)
	}
	if !sub.HasAttachment {
		c.Flash().Add("danger", "No attachment to download")
		return c.Redirect(302, "subIndexPath()", render.Data{"forum_title": c.Param("forum_title")})
	}
	w := c.Response()
	name := T.Translate(c, "app-submission-upload") + "-" + sub.ID.String()[0:8]
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, name))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(sub.Zip.ByteSlice)))

	if _, err := w.Write(sub.Zip.ByteSlice); err != nil {
		return c.Error(500, err)
	}
	return c.Redirect(200, "subResponseIndexPath()", render.Data{"forum_title": c.Param("forum_title"), "sid": c.Param("sid")})
}

// Check if user should be able to submit a form
func userSubmissionGreenlight(c buffalo.Context, user *models.User, sub *models.Submission) (okErr error) {
	R := unmarshalYaml(c, sub)
	for _, input := range *R {
		if input["type"].(string) == "require_final_evaluations" {
			okErr = evaluationsPassed(c, user)
		}
	}
	return okErr
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

// checking could be separated into functions :(
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
	castName, okName := theName.(string)
	if !okType || !okLabel || !okName {
		return fmt.Errorf(
			"Some attribute(s) is not a string for %q. Check YAML spec on types.\n"+
				"Passed: {type:%t, label:%t, name:%t}", castName, okType, okLabel, okName)
	}
	switch castType {
	case "dropdown":
		_, okOpts := r["options"]
		if !okOpts {
			return fmt.Errorf(
				"Did not find some required attribute(s) for %q in dropdown schema."+
					"Passed: {options:%t}", castName, okOpts)
		}
	case "file":
		_, okSize := r["max_size"]
		if !okSize {
			return fmt.Errorf(
				"Did not find some required attribute(s) for %q in file schema."+
					"Passed: {max_size:%t}", castName, okSize)
		}
	}
	return nil
}
