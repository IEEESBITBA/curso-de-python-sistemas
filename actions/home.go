package actions

import (
	"net/http"
	"time"

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
	c.Cookies().Set("auth_referer", c.Request().Referer(), time.Minute*5)
	return c.Render(http.StatusOK, r.HTML("auth.html"))
}
