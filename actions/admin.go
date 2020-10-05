package actions

import (
	"fmt"
	"time"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
)

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
		if auth.Key != authKey {
			c.Flash().Add("warning", "bad key")
			return c.Redirect(302, "controlPanelPath()")
		}
		c.Logger().Infof("user %s activated %s", usr.Email, url)
		return next(c)
	}
}

func downloadSQL(c buffalo.Context) error {
	w := c.Response()
	tstart := time.Now()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, App().Name))
	err := models.DBToJSON(w)
	if err != nil {
		return c.Error(500, err)
	}
	w.WriteHeader(200)
	c.Logger().Infof("sql -> json download time elapsed %s", time.Since(tstart))
	return nil
}
