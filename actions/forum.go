package actions

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"sort"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/pop/v5/slices"
	"github.com/pkg/errors"
	"github.com/soypat/curso/models"
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
			return c.Error(404,err)
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

func createForum(c buffalo.Context) error {
	return c.Render(200, r.HTML("forums/create.plush.html"))
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
		//return c.Render(200,r.HTML("forums/create.plush.html"))
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
		return &b, err
	}
	defer in.Close()
	b, err = ioutil.ReadAll(in)
	return &b, nil
}

func validURLDir(name string) bool {
	re := regexp.MustCompile(fmt.Sprintf(`[0-9a-zA-Z\-_]{%d}`, len(name)))
	return re.MatchString(name)
}
