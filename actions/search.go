package actions

import (
	"os"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/blevesearch/bleve"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

const indexName = "cursoP.bleve"

var bleveIndex bleve.Index

var theGreatNormalizer transform.Transformer

func init() {
	_ = os.Mkdir(indexName, os.ModeDir)
	_ = os.Chmod(indexName, 0666)
	theGreatNormalizer = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
}

// normalize converts extended unicode characters to their `normalized version`.
// examples: à -> a,   å -> a,   é -> e
func normalize(s string) string {
	result, _, _ := transform.String(theGreatNormalizer, s)
	return result
}

// runDBSearchIndex Indexes topics and replies every x minutes (30 minutes)
// for the search engine
func runDBSearchIndex() {
	l := App().Logger
	var err error
	bleveIndex, err = bleve.New(indexName, bleve.NewIndexMapping())
	switch err {
	case bleve.ErrorIndexPathExists:
		bleveIndex, err = bleve.Open(indexName)
	}
	if err != nil || bleveIndex == nil {
		os.Remove(indexName + "/store")
		os.Remove(indexName + "/index_meta.json")
		os.RemoveAll(indexName)
		l.Fatalf("at runDBSearchIndex(): bleve New index. removing all contents. please restart: %s", err)
	}

	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()

	run := func() {
		tstart := time.Now()
		err := indexDB()
		if err != nil {
			l.Errorf("bleve indexing DB: %s", err)
		}
		l.Printf("topic/reply indexing elapsed: %s", time.Since(tstart))
	}

	run()
	for range tick.C {
		run()
	}
}

func indexDB() error {
	l := App().Logger

	return models.DB.Transaction(func(tx *pop.Connection) error {

		topics := new(models.Topics)
		if err := tx.Where("deleted IS false").All(topics); err != nil {
			return errors.WithStack(err)
		}
		for _, t := range *topics {
			// if t.Deleted {
			// 	continue
			// }
			usr := new(models.User)
			if err := tx.Find(usr, t.AuthorID); err != nil {
				l.Errorf("'tx.Find(usr, %s)' FAILED in bleve.indexDB!", t.AuthorID)
				continue
			}
			t.Content = normalize(t.Content)
			t.Title = normalize(t.Title)
			t.Author = usr
			t.Author.Name = normalize(t.Author.Name)
			ID := bleveTopicID(&t, nil)
			err := bleveIndex.Index(ID, t)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		replies := new(models.Replies)
		if err := tx.Where("deleted IS false").All(replies); err != nil {
			return errors.WithStack(err)
		}
		for _, r := range *replies {
			if r.Deleted {
				continue
			}
			usr := new(models.User)
			if err := tx.Find(usr, r.AuthorID); err != nil {
				l.Errorf("'tx.Find(usr, %s)' FAILED in bleve.indexDB!", r.AuthorID)
				continue
			}
			t := new(models.Topic)
			if err := tx.Find(t, r.TopicID); err != nil {
				l.Errorf("'tx.Find(usr, %s)' FAILED in bleve.indexDB!", r.AuthorID)
				continue
			}
			if t.Deleted {
				continue
			}

			r.Content = normalize(r.Content)
			r.Author = usr
			r.Author.Name = normalize(r.Author.Name)
			ID := bleveTopicID(t, &r)
			err := bleveIndex.Index(ID, r)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	})
}

// TopicSearch handles search-link click event from the search result page
func TopicSearch(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	topic := &models.Topic{}
	q := tx.Where("id = ?", c.Param("tid"))
	if err := q.First(topic); err != nil {
		c.Flash().Add("danger", T.Translate(c, "topic-not-found"))
		return c.Error(404, err)
	}
	c.Logger().Infof("topicSearch with params:", c.Params())
	cat := &models.Category{}
	if err := tx.Find(cat, topic.CategoryID); err != nil {
		c.Flash().Add("danger", T.Translate(c, "category-not-found"))
		return c.Error(404, err)
	}
	forum := &models.Forum{}
	if err := tx.Find(forum, cat.ParentCategory); err != nil {
		c.Flash().Add("danger", T.Translate(c, "forum-not-found"))
		return c.Error(404, err)
	}
	return c.Redirect(302, "topicGetPath()", render.Data{"forum_title": forum.Title, "cat_title": cat.Title, "tid": topic.ID})
}

// Search handles and does heavylifting of search. renders search results page
func Search(c buffalo.Context) error {
	if c.Param("query") == "" {
		return c.Render(200, r.HTML("/search/search.plush.html"))
	}

	// query string is human syntax, see: http://blevesearch.com/docs/Query-String-Query/
	query := bleve.NewQueryStringQuery(normalize(c.Param("query")))
	req := bleve.NewSearchRequest(query)

	req.Size = 100
	req.Highlight = bleve.NewHighlight()
	q := bleve.NewMatchAllQuery()
	reqAll := bleve.NewSearchRequest(q)
	All, _ := bleveIndex.Search(reqAll)

	res, err := bleveIndex.Search(req)
	if err != nil {
		return errors.WithStack(err)
	}
	c.Logger().Infof("With \"%s\" Got results: %v\n From available: %v", c.Param("query"), res, All)
	c.Set("results", res)
	return c.Render(200, r.HTML("/search/search.plush.html"))
}

/*
 * the functions below are to provide
 * us with topic and reply information
 * once a match is found. This is done
 * by adding topic author, title information
 * into the bleve ID so it can be extracted once
 * it is found. schSS is a arbitrary separator
 */
const schSS = " __\000-__ "

// bleveTopicID Marshals topic or reply information into an unique
// representation. Used for generating bleve ID for indexing.
func bleveTopicID(t *models.Topic, r *models.Reply) string {
	if r != nil {
		return strings.Join([]string{t.Title, r.TopicID.String(), DisplayName(r.Author), r.ID.String()}, schSS)
	}
	return strings.Join([]string{t.Title, t.ID.String(), DisplayName(t.Author)}, schSS)
}

// bleveTopicFromID unmarshals topic/reply information from
// an ID string previously generated by bleveTopicID
func bleveTopicFromID(s string) *models.Topic {
	tsli := strings.Split(s, schSS)
	ID, _ := uuid.FromString(tsli[1])
	t := models.Topic{
		ID:     ID,
		Title:  tsli[0],
		Author: &models.User{Nick: tsli[2]},
	}
	if len(tsli) > 3 { // we have a reply on our hands
		rID, _ := uuid.FromString(tsli[3])
		t.Replies = []models.Reply{{ID: rID}}
	}
	return &t
}
