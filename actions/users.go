package actions

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
)

func UsersViewAllGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	users := &models.Users{}
	if err := tx.All(users); err != nil {
		return errors.WithStack(err)
	}
	c.Set("users", users)
	return c.Render(200, r.HTML("users/view-all.plush.html"))
}

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

func UserSettingsGet(c buffalo.Context) error {
	return c.Render(200, r.HTML("users/settings.plush.html"))
}

func UserSettingsPost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	userDB := c.Value("current_user").(*models.User)
	user := &models.User{}
	if err := c.Bind(user); err != nil {
		return errors.WithStack(err)
	}
	s, err := sanitizeNick(user.Nick)
	if err != nil {
		c.Flash().Add("danger", T.Translate(c, "user-settings-nick-invalid"))
		return c.Redirect(302, "userSettingsPath()")
	}
	c.Session().Set("code_theme", user.Theme)
	userDB.Nick = s
	if err := tx.Update(userDB); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "user-settings-edit-success"))
	return c.Redirect(302, "userSettingsPath()") //return c.Redirect(302, "topicGetPath()", render.Data{"forum_title":f.Title, "cat_title":c.Param("cat_title"),
	// "tid":c.Param("tid")})
}

func sanitizeNick(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > 10 {
		return "", errors.New("nick too long")
	}
	re := regexp.MustCompile(fmt.Sprintf("[0-9a-zA-Z]{%d}", len(s)))
	if !re.MatchString(s) {
		return s, errors.New("nick contains non alphanumeric char")
	}
	return s, nil
}
