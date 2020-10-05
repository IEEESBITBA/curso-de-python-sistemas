package models

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"go.etcd.io/bbolt"

	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/pop/v5"
)

// DB is a connection to your database to be used
// throughout your application.
var DB *pop.Connection

// BDB is a transaction in the simple BBolt database
// for fulfilling database needs when a full blown
// SQL server is not required
var BDB *bbolt.DB

func init() {
	// SQL initialization
	var err error
	env := envy.Get("GO_ENV", "development")
	DB, err = pop.Connect(env)
	if err != nil {
		log.Fatal(err)
	}
	pop.Debug = env == "development"

	// Begin BBolt initialization
	_ = os.Mkdir("tmp", os.ModeDir) // make directory just in case
	BDB, err = bbolt.Open("tmp/bbolt.db", 0600, &bbolt.Options{Timeout: 4 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	// any new bucket names must be created here
	_ = BDB.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("pyUploads"))
		must(err)
		_, err = tx.CreateBucketIfNotExists([]byte("safeUsers"))
		must(err)
		return nil
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// DBToJSON encodes the site's SQL structure to json
func DBToJSON(w io.Writer) error {
	usrs := new(Users)
	if err := DB.All(usrs); err != nil {
		return err
	}
	type Repl struct {
		Content  string `json:"content"`
		AuthorID string `json:"author_id"`
	}
	type Top struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		AuthorID string `json:"author_id"`
		Repls    []Repl `json:"replies"`
	}
	type Cat struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Tops        []Top  `json:"topics"`
	}
	type Frum struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Cats        []Cat
	}
	type Eval struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Solution    string `json:"solution"`
		Inputs      string `json:"stdin"`
		ID          string `json:"ID"`
	}

	site := make(map[string]interface{})
	site["forums"] = []Frum{}
	site["users"] = usrs
	site["evaluations"] = []Eval{}
	forums := new(Forums)
	categories := new(Categories)
	topics := new(Topics)
	replies := new(Replies)
	evaluations := new(Evaluations)
	if err := DB.All(forums); err != nil {
		return err
	}
	if err := DB.All(evaluations); err != nil {
		return err
	}
	for _, e := range *evaluations {

		eval := Eval{Title: e.Title, Description: e.Description, Content: e.Content, Solution: e.Solution, Inputs: e.Inputs.String, ID: e.ID.String()}
		site["evaluations"] = append(site["evaluations"].([]Eval), eval)
	}
	for _, f := range *forums {
		frum := Frum{Title: f.Title, Description: f.Description}
		if err := DB.Where("parent_category = ?", f.ID).All(categories); err != nil {
			return err
		}
		for _, c := range *categories {
			cat := Cat{Title: c.Title, Description: c.Description.String}
			if err := DB.BelongsTo(&c).All(topics); err != nil {
				return err
			}
			for _, t := range *topics {
				topAuthor := new(User)
				_ = DB.Where("id = ?", t.AuthorID).First(topAuthor) // if not found, keep going
				top := Top{Title: t.Title, Content: t.Content, AuthorID: t.AuthorID.String()}
				if err := DB.BelongsTo(&t).All(replies); err != nil {
					return err
				}
				for _, r := range *replies {
					replAuthor := new(User)
					_ = DB.Where("id = ?", r.AuthorID).First(replAuthor) // if not found, keep going
					top.Repls = append(top.Repls, Repl{Content: r.Content, AuthorID: r.AuthorID.String()})
				}
				cat.Tops = append(cat.Tops, top)
			}
			frum.Cats = append(frum.Cats, cat)
		}
		site["forums"] = append(site["forums"].([]Frum), frum)
	}

	j := json.NewEncoder(w)
	j.SetIndent("", "\t")
	return j.Encode(site)
}
