package actions

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"
	"strings"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/pop/v5/slices"
	"github.com/pkg/errors"
)

// SetCurrentForum attempts to find a forum definition in the db.
func SetCurrentForum(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		tx := c.Value("tx").(*pop.Connection)
		forum := &models.Forum{}
		title := c.Param("forum_title")
		if title == "" {
			return c.Redirect(200, "/f")
		}
		q := tx.Where("title = ?", title)
		err := q.First(forum)
		if err != nil {
			c.Flash().Add("danger", "Error while seeking forum")
			return c.Error(404, err)
		}
		c.Set("inForum", true)
		c.Set("forum", forum)
		return next(c)
	}
}

func forumIndex(c buffalo.Context) error {
	forum := c.Value("forum").(*models.Forum)
	tx := c.Value("tx").(*pop.Connection)
	cats := &models.Categories{}
	//if err := tx.BelongsTo(forum).All(cats); err != nil {
	//	return c.Error(404, err)
	//}

	q := tx.PaginateFromParams(c.Params()).Where("parent_category = ?", forum.ID)
	err := q.All(cats)
	if err != nil {
		return errors.WithStack(err)
	}
	sort.Sort(cats)
	c.Set("categories", cats)
	c.Set("pagination", q.Paginator)
	return c.Render(200, r.HTML("forums/index.plush.html"))
}

func CreateEditForum(c buffalo.Context) error {
	return c.Render(200, r.HTML("forums/create.plush.html"))
}
func EditForumPost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	originalForum := &models.Forum{}
	q := tx.Where("title = ?", c.Param("forum_title"))
	if err := q.First(originalForum); err != nil {
		c.Flash().Add("danger", "internal error. could not find forum")
		return c.Error(500, err)
	}
	f := &models.Forum{}
	if err := c.Bind(f);  err != nil && !strings.Contains(err.Error(), "could not parse number") {
		c.Flash().Add("danger", "could not edit forum"+err.Error())
		return c.Error(400, err)
	}
	if !validURLDir(f.Title) {
		c.Flash().Add("danger", "Forum title should contain url safe characters (A-Z,a-z,-,_)")
		return c.Redirect(302, "/admin/f/")
	}
	originalForum.Title, originalForum.Description, originalForum.Defcon = f.Title, f.Description, f.Defcon
	b, _ := getFormFile(c, "logo")
	if b != nil {
		originalForum.Logo = *b
	}
	if _,err:=tx.ValidateAndUpdate(originalForum); err!=nil {
		c.Flash().Add("danger", "error validating or saving")
		return c.Redirect(500, "/admin/f/")
	}
	c.Flash().Add("success", fmt.Sprintf("Forum %s edited", originalForum.Title))
	return c.Redirect(302, "/admin/f")
}

func createForumPost(c buffalo.Context) error {
	f := &models.Forum{}
	if err := c.Bind(f); err != nil {
		c.Flash().Add("danger", "could not create forum")
		return c.Error(400, err)
	}
	b, err := getFormFile(c, "logo")
	if err != nil {
		c.Flash().Add("danger", "error reading logo image")
		return c.Redirect(302, "/admin/f/")
	}
	if !validURLDir(f.Title) {
		c.Flash().Add("danger", "Forum title should contain url safe characters (A-Z,a-z,-,_)")
		return c.Redirect(302, "/admin/f/")
	}
	f.Staff = slices.UUID{}
	f.Logo = *b
	tx := c.Value("tx").(*pop.Connection)
	q := tx.Where("title = ?", f.Title)
	exist, err := q.Exists(&models.Forum{})
	if exist {
		c.Flash().Add("danger", "Forum already exists")
		return c.Redirect(302, "/admin/f/")
	}
	v, _ := f.Validate(tx)
	if v.HasAny() {
		c.Flash().Add("danger", "Title and description should have something!")
		return c.Redirect(302, "/admin/f/")
	}
	err = tx.Save(f)
	if err != nil {
		c.Flash().Add("danger", "Error creating forum")
		//return c.Render(200,r.HTML("forums/create.plush.html"))
		return errors.WithStack(err)
	}
	c.Flash().Add("success", fmt.Sprintf("Forum %s created", f.Title))
	return c.Redirect(302, "/admin/f")
}

func manageForum(c buffalo.Context) error {
	//forum := c.Value("forum").(*models.Forum)
	tx := c.Value("tx").(*pop.Connection)
	forums := &models.Forums{}
	q := tx.PaginateFromParams(c.Params())
	c.Set("pagination", q.Paginator)
	if err := q.All(forums); err != nil {
		c.Logger().Warn("Error looking for forums in manageForum. Maybe no forums present?")
		c.Set("forums", forums)
		return c.Render(200, r.HTML("forums/manage.plush.html"))
	}
	//sort.Sort(forums) // TODO implement sort interface for forum
	c.Set("forums", forums)
	return c.Render(200, r.HTML("forums/manage.plush.html"))
}

func getFormFile(c buffalo.Context, key string) (*[]byte, error) {
	var b []byte
	in, _, err := c.Request().FormFile(key)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	b, err = ioutil.ReadAll(in)
	if err!=nil || len(b) < 2 {
		return nil, err
	}
	if len(b) > 100 { // we discard xml start of svg file
		svgIdx := strings.Index(string(b[:100]), "<svg ")
		if svgIdx > 0 {
			b = b[svgIdx:]
		}
	}
	return &b, nil
}

func validURLDir(name string) bool {
	re := regexp.MustCompile(fmt.Sprintf(`[0-9a-zA-Z\-_]{%d}`, len(name)))
	return re.MatchString(name)
}
