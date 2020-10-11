package actions

import (
	"fmt"
	"os"
	"time"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
)

func init() {
	_ = os.MkdirAll("assets/server/data", os.ModeDir)
}

// UsersViewAllGet renders all users page (admins only)
func UsersViewAllGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	users := &models.Users{}
	var q *pop.Query
	if c.Param("emailquery") != "" {
		q = tx.Where("email = ?", c.Param("emailquery")).PaginateFromParams(c.Params())
		c.Logger().Infof("emailqueried %s", c.Param("emailquery"))
	} else {
		q = tx.Order("role ASC").PaginateFromParams(c.Params())
	}
	if c.Param("per_page") == "" { // set default max results per page if not set
		q.Paginator.PerPage = 20
	}

	if err := q.All(users); err != nil {
		c.Logger().Errorf("UsersViewAllGet %s: %s", c.Param("emailquery"), err)
		return errors.WithStack(err)
	}
	c.Set("users", users)
	c.Set("pagination", q.Paginator)
	return c.Render(200, r.HTML("users/view-all.plush.html"))
}

// AdminUserGet handles the event when an admin is created by another admin
func AdminUserGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	adminuser := &models.User{}
	q := tx.Where("id = ?", c.Param("uid"))
	if err := q.First(adminuser); err != nil {
		return errors.WithStack(err)
	}
	adminuser.Role = "admin"
	if err := tx.Update(adminuser); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", fmt.Sprintf("user %s is now admin", adminuser.Name))
	return c.Redirect(302, "allUsersPath()")
}

// NormalizeUserGet event removing status from user and setting to empty string
func NormalizeUserGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	adminuser := &models.User{}
	q := tx.Where("id = ?", c.Param("uid"))
	if err := q.First(adminuser); err != nil {
		return errors.WithStack(err)
	}
	adminuser.Role = ""
	if err := tx.Update(adminuser); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", fmt.Sprintf("user %s has been normalized", adminuser.Name))
	return c.Redirect(302, "allUsersPath()")
}

// BanUserGet ban user event by admin
func BanUserGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	banuser := &models.User{}
	q := tx.Where("id = ?", c.Param("uid"))
	if err := q.First(banuser); err != nil {
		return errors.WithStack(err)
	}
	banuser.Role = "banned"
	if err := tx.Update(banuser); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", fmt.Sprintf("user %s banned", banuser.Name))
	return c.Redirect(302, "allUsersPath()")

}

// ControlPanel renders page for controlling server backend stuff.
// html contains python deletion at the time of writing this
func ControlPanel(c buffalo.Context) error {
	return c.Render(200, r.HTML("curso/control-panel.plush.html"))
}

// ControlPanelHandler handles POSTs from controlpanel
// forms. It requests an auth key and if it fails to
// validate it errors
func ControlPanelHandler(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		usr, ok := c.Value("current_user").(*models.User)
		url := c.Request().URL.String()
		if !ok || usr.Role != "admin" {
			c.Flash().Add("danger", "User not fount")
			c.Logger().Errorf("controlPanelHandler @%s on current_user:%v on %s", url, usr)
			return c.Redirect(504, "/")
		}
		var auth struct {
			Key string `form:"authkey"`
		}
		if err := c.Bind(&auth); err != nil {
			return c.Error(500, err)
		}
		if auth.Key != "1337" {
			c.Flash().Add("warning", "bad key")
			return c.Redirect(302, "controlPanelPath()")
		}
		c.Logger().Infof("user %s activated %s", usr.Email, url)
		return next(c)
	}
}

var indexing bool
var jobname string

func generateJSONFromSQL(c buffalo.Context) error {
	if indexing {
		c.Flash().Add("danger", "currently on job "+jobname)
		return c.Redirect(302, "controlPanelPath()", render.Data{})
	}
	tstart := time.Now()
	jobname = "assets/server/data/sql_" + tstart.Format("Jan_2_15:04:05_2006") + ".json"
	f, err := os.Create(jobname)
	if err != nil {
		return c.Error(500, err)
	}
	go func() {
		defer f.Close()
		if indexing {
			return
		}
		indexing = true
		models.DBToJSON(f)
		indexing = false
		app.Logger.Infof("%s generation time elapsed %s", jobname, time.Since(tstart))
	}()
	c.Flash().Add("info", fmt.Sprintf("sql file is processing. will be available at %s/%s. May take a few seconds", app.Host, jobname))
	return c.Redirect(302, "controlPanelPath()", render.Data{})
}
