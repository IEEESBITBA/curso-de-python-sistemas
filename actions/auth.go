package actions

import (
	"fmt"
	"os"
	"strings"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/gofrs/uuid"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/pkg/errors"
)

const cookieUIDName = "current_user_id"

func init() {
	gothic.Store = App().SessionStore
	goth.UseProviders(
		google.New(os.Getenv("GGL_KEY_FORUM"), os.Getenv("GGL_SECRET_FORUM"), fmt.Sprintf("%s%s", App().Host, "/auth/google/callback"),
			"profile", "email"),
		facebook.New(os.Getenv("FB_KEY_FORUM"), os.Getenv("FB_SECRET_FORUM"), fmt.Sprintf("%s%s", App().Host, "/auth/facebook/callback"),
			"public_profile", "email"),
		github.New(os.Getenv("GITHUB_KEY_FORUM"), os.Getenv("GITHUB_SECRET_FORUM"), fmt.Sprintf("%s%s", App().Host, "/auth/github/callback")),
	)
}

// AuthCallback When user log into provider they are redirected
// to this function which creates the session id
// in user cookie jar. The user then can then be
// authorized successfully through Authorize function
// The user is also added to DB if they don't exist here
func AuthCallback(c buffalo.Context) error {
	c.Logger().Debug("AuthCallback called")
	redirectURL, err := c.Cookies().Get("auth_referer")
	if redirectURL == "" || err == nil {
		redirectURL = "/"
	}
	gu, err := gothic.CompleteUserAuth(c.Response(), c.Request())
	if err != nil {
		c.Flash().Add("danger", T.Translate(c, "app-auth-error"))
		return c.Redirect(302, redirectURL) //c.Error(401, err)
	}
	if strings.TrimSpace(gu.Email) == "" || strings.Index(gu.Email, "@") <= 0 {
		c.Flash().Add("warning", T.Translate(c, "app-user-auth-empty-email", render.Data{"provider": gu.Provider}))
		return c.Redirect(302, redirectURL)
	}
	tx := c.Value("tx").(*pop.Connection)
	q := tx.Where("provider = ? and provider_id = ?", gu.Provider, gu.UserID)
	exists, err := q.Exists("users") // look for an entry with matching providers and userID
	if err != nil {
		return errors.WithStack(err)
	} // check users table exists
	u := &models.User{}
	if exists { // if we find our user, save data to `u`
		err = q.First(u)
		if err != nil {
			return errors.WithStack(err)
		}
	} else { // if we don't find user, create new user!
		if gu.Name == "" {
			gu.Name = "usuario"
		}
		u.Name = strings.Title(strings.ToLower(gu.Name))
		u.Email = strings.ToLower(gu.Email)
		u.Provider = gu.Provider
		u.ProviderID = gu.UserID
		u.AvatarURL = gu.AvatarURL
		c.Logger().Infof("Creating new user! Email: %s", gu.Email)
		err = tx.Save(u)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	c.Session().Set(cookieUIDName, u.ID) // This line sets user cookie for future Authorize callbacks to verify successfully
	err = c.Session().Save()
	if err != nil {
		return errors.WithStack(err)
	}

	c.Flash().Add("success", T.Translate(c, "app-login"))
	// Do something with the user, maybe register them/sign them in
	c.Logger().Debug("AuthCallback finished successfully")
	return c.Redirect(302, redirectURL) // redirect to homepage
}

// AuthDestroy logout process. kills cookies leaving user
// unable to Authorize without logging in again
func AuthDestroy(c buffalo.Context) error {
	c.Cookies().Delete(App().SessionName)
	c.Session().Clear()
	err := c.Session().Save()
	if err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "app-logout"))

	return c.Redirect(302, c.Request().Referer())
}

// Authorize Backbone of the authorization process.
// This should run before displaying any internal page
// and kick unauthorized users back to homepage
func Authorize(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		c.Logger().Debug("Authorize called")
		unverifiedUID := c.Session().Get(cookieUIDName)
		if unverifiedUID == nil {
			c.Flash().Add("danger", T.Translate(c, "app-user-required"))
			return c.Redirect(302, "/")
		}
		uid := unverifiedUID.(uuid.UUID)
		tx := c.Value("tx").(*pop.Connection)
		q := tx.Where("id = ?", uid)
		exists, err := q.Exists("users")
		if err != nil {
			return c.Redirect(500, "/")
		}
		if !exists {
			c.Flash().Add("danger", T.Translate(c, "app-user-auth-error"))
			return AuthDestroy(c)
		}
		u := &models.User{}
		err = q.First(u)
		if err != nil {
			return c.Redirect(500, "/")
		}
		c.Set("username", u.Name)
		c.Logger().Debugf("Finished Authorize. %s authorized", u.Name)
		return next(c)
	}
}

// SetCurrentUser This function is to provide Context with user information on `current_user`.
// If user is not logged in it does nothing.
func SetCurrentUser(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		c.Logger().Debug("SetCurrentUser called.")
		c.Set("role", "")
		if uid := c.Session().Get(cookieUIDName); uid != nil {
			c.Logger().Debugf("uid:%s found in SetCurrentUser", uid)
			u := &models.User{}
			tx := c.Value("tx").(*pop.Connection)
			err := tx.Find(u, uid)
			if err != nil {
				c.Logger().Errorf("setCurrentUser with uid %s: %s", uid, err)
				return next(c)
			}
			if u.Role == "banned" {
				return c.Redirect(302, "https://ieeeitba.web.app/cursospython")
			}
			theme := c.Session().Get("code_theme")
			if theme == nil {
				theme = ""
			}
			u.Theme = fmt.Sprintf("%s", theme)
			c.Set("current_user", u)
			c.Set("role", u.Role)
			c.Logger().Debugf("SetCurrentUser success auth:%s", u.Email)
		}
		return next(c)
	}
}

// AdminAuth This authorization is for server maintenance/management only
// authorizes where user has role=='admin'
func AdminAuth(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		c.Logger().Debug("AdminAuth called")
		if uid := c.Session().Get(cookieUIDName); uid != nil {
			u := &models.User{}
			tx := c.Value("tx").(*pop.Connection)
			c.Logger().Debug(uid.(uuid.UUID).String())
			q := tx.Where("id = ?  and role = ?", uid.(uuid.UUID).String(), "admin") // FIXME check provider too for increased security
			exists, err := q.Exists(u)
			if err != nil {
				return c.Error(404, err)
			}
			if exists {
				c.Logger().Infof("AdminAuth success: %s", u.Email)
				return next(c) // user has admin role
			}
		}
		c.Flash().Add("danger", "You can't do that!")
		return c.Redirect(403, "/") // user not found in db or does not have admin role
	}
}
