package actions

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"go.etcd.io/bbolt"
)

var (
	errBucketNotFound = errors.New("did not find " + safeUsersBucketName + " bucket in bolt database")
)

// name must be first created in models/bbolt.go in init()
const safeUsersBucketName = "safeUsers"

type safeUser struct {
	Name        string `json:"nick" gob:"nick"`
	Email       string `json:"email" gob:"email"`
	Responsible string `json:"responsible" gob:"resp"`
}

type safeUsers []safeUser

func (s safeUsers) String() (out string) {
	for _, u := range s {
		out += u.Email + "\n"
	}
	return
}

var (
	words   []string
	teamIDs map[string]int
)

func init() {
	f, err := os.Open("data/badwords.es.en.txt")
	must(err)
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	must(err)
	words = strings.Split(string(b), "\n")
	f.Close()
	f, err = os.Open("data/teamids.tsv")
	must(err)
	r := csv.NewReader(f)
	r.Comma = '\t'
	records, err := r.ReadAll()
	must(err)
	teamIDs = make(map[string]int)
	for _, r := range records[1:] { // exclude header
		if len(r) != 2 {
			must(fmt.Errorf("error reading teamid records %q", r))
		}
		id, err := strconv.Atoi(r[0])
		must(err)
		if !isEmail(strings.TrimSpace(r[1])) {
			must(fmt.Errorf("mail %q is not valid", r[1]))
		}
		teamIDs[r[1]] = id
	}
}

// SafeListGet renders page with safelist. only admins can see
func SafeListGet(c buffalo.Context) error {
	var users safeUsers
	btx := c.Value("btx").(*bbolt.Tx)
	err := func() error {
		b := btx.Bucket([]byte(safeUsersBucketName))
		if b == nil {
			return errBucketNotFound
		}
		return b.ForEach(func(_, v []byte) error {
			var user safeUser
			err := json.Unmarshal(v, &user)
			if err != nil {
				return err
			}
			users = append(users, user)
			return nil
		})
	}()

	if err != nil {
		return c.Error(500, err)
	}
	c.Set("safe_users", users)
	return c.Render(200, r.HTML("users/safelist.plush.html"))
}

type safeForm struct {
	List string `json:"safelist" form:"safelist"`
}

// SafeListPost handles event when more emails are added to safelist
func SafeListPost(c buffalo.Context) error {
	responsible := c.Value("current_user").(*models.User)
	var form safeForm
	if err := c.Bind(&form); err != nil {
		return c.Error(500, err)
	}
	// make sure email is in lowercase to avoid dupes and false negatives in safelist matches
	form.List = strings.ToLower(form.List)
	users := safeFormToSafeList(form)
	btx := c.Value("btx").(*bbolt.Tx)
	err := func() error {
		b := btx.Bucket([]byte(safeUsersBucketName))
		if b == nil {
			return errBucketNotFound
		}
		for _, user := range users {
			user.Responsible = responsible.Name
			bson, err := json.Marshal(user)
			if err != nil {
				return err
			}
			err = b.Put([]byte(user.Email), bson)
			if err != nil {
				return err
			}
		}
		return nil
	}()
	if err != nil {
		return c.Error(500, err)
	}
	c.Flash().Add("success", "Safelist updated successfully.")
	return c.Redirect(302, "allUsersPath()")
}

const safeDomain = "itba.edu.ar"

// SafeList works kind of like authorize but does not verify user exists.
//includes helper function "hasBadWord" for sanitizing dirty language
func SafeList(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		c.Set("hasBadWord", hasBadWord)
		u, ok := c.Value("current_user").(*models.User)
		if !ok || u == nil {
			c.Flash().Add("danger", T.Translate(c, "app-user-required"))
			return c.Redirect(302, "/")
		}
		if u.Role == "safe" || u.Role == "admin" {
			id, ok := teamIDs[u.Email]
			if ok {
				c.Set("team_id", id)
			}
			return next(c)
		}
		email := strings.ToLower(u.Email)
		two := strings.Split(email, "@")
		if two[1] == safeDomain {
			_ = setUserSafeRole(c, u)
			return next(c)
		}
		var exists bool
		btx := c.Value("btx").(*bbolt.Tx)
		err := btx.DB().View(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(safeUsersBucketName))
			if b == nil {
				return errBucketNotFound
			}
			exists = b.Get([]byte(email)) != nil
			return nil
		})
		if err != nil {
			c.Logger().Errorf("CRITICAL safelist malufunction: %s", err)
			return next(c)
		}
		if !exists {
			c.Logger().Warnf("safelist not contain %s", email)
			c.Flash().Add("warning", T.Translate(c, "safelist-user-not-found"))
			c.Session().Clear()
			_ = c.Session().Save()
			return c.Redirect(302, "/") //, render.Data{"provider":user.Provider})
		}
		if u.Role == "" {
			_ = setUserSafeRole(c, u)
		}
		return next(c)
	}
}

// setUserSafeRole updates DB with user role as 'safe'
func setUserSafeRole(c buffalo.Context, u *models.User) error {
	if u.Role != "" {
		c.Logger().Warnf("cant set '%s' role to 'safe' for %s", u.Role, u.Email)
		return nil
	}
	tx := c.Value("tx").(*pop.Connection)
	u.Role = "safe"
	c.Set("current_user", u)
	if err := tx.UpdateColumns(u, "role"); err != nil {
		c.Logger().Errorf("adding 'safe' role to user %s", u.Email)
		return err
	}
	return nil
}

// Parses form list from the form Post event
func safeFormToSafeList(sf safeForm) (SU safeUsers) {
	splits := strings.Split(sf.List, "\n")
	splitComma := strings.Split(sf.List, ",")
	splitSColon := strings.Split(sf.List, ";")
	if len(splitComma) > len(splits) {
		splits = splitComma
	}
	if len(splitSColon) > len(splits) && len(splitSColon) > len(splitComma) {
		splits = splitSColon
	}
	for _, email := range splits {
		email = strings.TrimSpace(email)
		if !isEmail(email) {
			continue
		}
		SU = append(SU, safeUser{
			Name:  "None",
			Email: email,
		})
	}
	return
}

func isEmail(s string) bool {
	if strings.ContainsAny(s, "\"(),:;<>[\\] \t\n") {
		return false
	}
	dub := strings.Split(s, "@")
	if len(dub) != 2 {
		return false
	}
	back := strings.Split(dub[1], ".")
	return len(back) >= 2
}

func hasBadWord(s string) string {
	if words == nil {
		return ""
	}
	for _, w := range words {
		w = strings.Trim(w, "*. ")
		if strings.Contains(s, w) {
			return w
		}
	}
	return ""
}
