package actions

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
)

// UsersViewAllGet renders all users page (admins only)
func UsersViewAllGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	users := &models.Users{}
	if c.Param("emailquery") != "" {
		if err := tx.Where("email = ?", c.Param("emailquery")).All(users); err != nil {
			c.Logger().Errorf("searching for %s:%s", c.Param("emailquery"), err)
			return c.Error(500, err)
		}
		c.Logger().Infof("emailqueried %s", c.Param("emailquery"))
		c.Set("users", users)
		return c.Render(200, r.HTML("users/view-all.plush.html"))
	}

	q := tx.Order("role ASC").PaginateFromParams(c.Params())
	if c.Param("per_page") == "" { // set default max results per page if not set
		q.Paginator.PerPage = 20
	}

	if err := q.All(users); err != nil {
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

// UserSettingsGet render page with user settings :)
func UserSettingsGet(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	topics := &models.Topics{}
	if err := tx.All(topics); err != nil {
		return errors.WithStack(err)
	}
	c.Set("topics", topics)
	return c.Render(200, r.HTML("users/settings.plush.html"))
}

// UserSettingsPost handles event when user changes setting by submitting setting form
func UserSettingsPost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	userDB := c.Value("current_user").(*models.User)
	user := &models.User{}
	if err := c.Bind(user); err != nil {
		return errors.WithStack(err)
	}
	s, err := sanitizeNick(user.Nick)
	emptyIfClean := hasBadWord(s)
	if emptyIfClean != "" {
		c.Logger().Infof("Bad word inserted by user %s: %s", user.Email, emptyIfClean)
		c.Flash().Add("warning", "@#%$*!!")
		return c.Redirect(302, "userSettingsPath()")
	}
	if err != nil {
		c.Flash().Add("danger", T.Translate(c, "user-settings-nick-invalid"))
		return c.Redirect(302, "userSettingsPath()")
	}
	c.Session().Set("code_theme", user.Theme)
	userDB.Nick = s
	if err := tx.UpdateColumns(userDB, "nick"); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "user-settings-edit-success"))
	return c.Redirect(302, "userSettingsPath()") //return c.Redirect(302, "topicGetPath()", render.Data{"forum_title":f.Title, "cat_title":c.Param("cat_title"),
	// "tid":c.Param("tid")})
}

// sanitizeNick don't want bad characters that break urls or weird stuff.
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

// UsersSettingsRemoveTopicSubscription handles user clicking remove subscription button
func UsersSettingsRemoveTopicSubscription(c buffalo.Context) error {
	var unsub []string
	usr := new(models.User)
	tx := c.Value("tx").(*pop.Connection)
	if err := tx.Find(usr, c.Param("uid")); err != nil {
		return errors.WithStack(err)
	}
	if c.Param("tid") == "all" {
		for _, sub := range usr.Subscriptions {
			topic := new(models.Topic)
			if err := tx.Find(topic, sub); err != nil {
				continue
			}
			unsub = append(unsub, topic.ID.String())
		}
		if len(unsub) == 0 {
			c.Flash().Add("warning", T.Translate(c, "topic-unsubscribe-empty"))
			return c.Redirect(302, "/u")
		}
	} else {
		unsub = append(unsub, c.Param("tid"))
	}
	for _, id := range unsub {
		topic := new(models.Topic)
		if err := tx.Find(topic, id); err != nil {
			return errors.WithStack(err)
		}
		usr.RemoveSubscription(topic.ID)
		if err := tx.UpdateColumns(usr, "subscriptions"); err != nil {
			return errors.WithStack(err)
		}
		topic.RemoveSubscriber(usr.ID)
		if err := tx.UpdateColumns(topic, "subscribers"); err != nil {
			return errors.WithStack(err)
		}
	}
	c.Flash().Add("success", T.Translate(c, "topic-unsubscribe-success"))
	return c.Redirect(302, "/u")
}
