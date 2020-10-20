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
	"github.com/gofrs/uuid"
)

const nullUUID = "00000000-0000-0000-0000-000000000000"

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
	site := make(map[string]interface{})
	usrs := new(Users)
	if err := DB.All(usrs); err != nil {
		return err
	}
	site["users"] = usrs

	type Repl struct {
		Content  string `json:"content"`
		AuthorID string `json:"author_id"`
	}
	type Top struct {
		id       uuid.UUID
		Voters   string  `json:"voters"`
		Votes    int     `json:"votes"`
		Archived bool    `json:"archived"`
		Deleted  bool    `json:"deleted"`
		Title    string  `json:"title"`
		Content  string  `json:"content"`
		AuthorID string  `json:"author_id"`
		Repls    []*Repl `json:"replies"`
	}
	type Cat struct {
		id          uuid.UUID
		Title       string `json:"title"`
		Description string `json:"description"`
		Tops        []*Top `json:"topics"`
	}
	type Frum struct {
		id          uuid.UUID
		Title       string `json:"title"`
		Description string `json:"description"`
		Cats        []*Cat `json:"categories"`
	}
	type Eval struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Solution    string `json:"solution"`
		Inputs      string `json:"stdin"`
		ID          string `json:"ID"`
	}

	frumsSlice := []*Frum{}
	evalsSlice := []*Eval{}
	forums := new(Forums)
	categories := new(Categories)
	topics := new(Topics)
	replies := new(Replies)
	evaluations := new(Evaluations)
	if err := DB.All(evaluations); err != nil {
		return err
	}
	for _, e := range *evaluations {
		evalsSlice = append(evalsSlice, &Eval{Title: e.Title, Description: e.Description, Content: e.Content, Solution: e.Solution, Inputs: e.Inputs.String, ID: e.ID.String()})
	}
	site["evaluations"] = evalsSlice
	if err := DB.All(forums); err != nil {
		return err
	}
	if err := DB.All(categories); err != nil {
		return err
	}
	if err := DB.All(topics); err != nil {
		return err
	}
	if err := DB.All(replies); err != nil {
		return err
	}
	for _, f := range *forums {
		frumsSlice = append(frumsSlice, &Frum{Title: f.Title, Description: f.Description, id: f.ID})
	}

	for _, f := range frumsSlice {
		for _, c := range *categories {
			if c.ParentCategory.UUID == f.id {
				f.Cats = append(f.Cats, &Cat{Title: c.Title, Description: c.Description.String, id: c.ID})
				continue
			}
		}
	}
	for _, f := range frumsSlice {
		for _, c := range f.Cats {
			for _, t := range *topics {
				if c.id == t.CategoryID {
					c.Tops = append(c.Tops, &Top{Title: t.Title, Content: t.Content, AuthorID: t.AuthorID.String(),
						id: t.ID, Deleted: t.Deleted, Archived: t.Archived, Votes: t.Votes(), Voters: t.Voters.Format(";")})
					continue
				}
			}
		}
	}
	for _, f := range frumsSlice {
		for _, c := range f.Cats {
			for _, t := range c.Tops {
				for _, r := range *replies {
					if r.TopicID == t.id {
						t.Repls = append(t.Repls, &Repl{Content: r.Content, AuthorID: r.AuthorID.String()})
						continue
					}
				}
			}
		}
	}
	site["forums"] = frumsSlice
	j := json.NewEncoder(w)
	j.SetIndent("", "\t")
	return j.Encode(site)
}
