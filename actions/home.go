package actions

import (
	"fmt"
	"net/http"
	"time"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
)

// HomeHandler is a default handler to serve up
// a home page. unused.
func HomeHandler(c buffalo.Context) error {
	return c.Render(http.StatusOK, r.HTML("index.html"))
}

// SiteStruct adds basic paths to context. is legacy code
// and should be removed in favour of using context functions
// such as forumPath()
func SiteStruct(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		c.Set("forumBase", "/f")
		c.Set("categories_index", "/c")
		c.Set("user_settings_path", "/u")
		c.Set("auth_path", "/auth")
		c.Set("root_path", "/")
		c.Set("search_path", "/s")
		c.Set("inForum", false)
		c.Set("create_forum_path", "/admin/newforum/")

		return next(c)
	}
}

// AuthHome renders page with all provider options
func AuthHome(c buffalo.Context) error {
	return c.Render(http.StatusOK, r.HTML("auth.html"))
}

func downloadSQL(c buffalo.Context) error {
	var auth struct {
		Key string `form:"authkey"`
	}
	if err := c.Bind(&auth); err != nil {
		return c.Error(500, err)
	}
	if auth.Key != authKey {
		c.Flash().Add("warning", "bad key")
		return c.Redirect(302, "controlPanelPath()")
	}
	w := c.Response()
	tstart := time.Now()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, App().Name))
	// w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
	err := models.DBToJSON(w)

	if err != nil {
		return c.Error(500, err)
	}
	w.WriteHeader(200)
	c.Logger().Infof("sql -> json download time elapsed %s", time.Since(tstart))
	return nil

}
