package actions

import (
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/blevesearch/bleve"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"os"
	"strings"
	"time"
)

const indexName = "cursoP.bleve"

var bleveIndex bleve.Index

func init() {
	os.Mkdir(indexName,os.ModeDir)
	os.Chmod(indexName,0666)
}

func runDBSearchIndex() {
	l := App().Logger
	var err error
	bleveIndex, err = bleve.New(indexName, bleve.NewIndexMapping())
	switch err {
	case bleve.ErrorIndexPathExists:
		bleveIndex, err = bleve.Open(indexName)
	}
	if err != nil || bleveIndex == nil{
		os.Remove(indexName+"/store")
		os.Remove(indexName+"/index_meta.json")
		os.RemoveAll(indexName)
		l.Fatalf("at runDBSearchIndex(): bleve New index. removing all contents. please restart: %s",err)
	}

	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()

	run := func() {
		err := indexDB()
		if err != nil {
			l.Errorf("bleve indexing DB: %s", err)
		}
	}

	run()
	for range tick.C {
		run()
	}
}

func indexDB() error {
	l := App().Logger
	type indexedTopic struct {
		ID      uuid.UUID
		Title   string
		Content string
	}

	type indexedReply struct {
		ID      uuid.UUID
		Content string
	}

	return models.DB.Transaction(func(tx *pop.Connection) error {

		topics := new(models.Topics)
		if err := tx.All(topics); err != nil {
			return errors.WithStack(err)
		}
		for _, t := range *topics {
			if t.Deleted {
				continue
			}
			usr := new(models.User)
			if err := tx.Find(usr, t.AuthorID); err != nil {
				l.Errorf("'tx.Find(usr, %s)' FAILED in bleve.indexDB!", t.AuthorID)
				continue
			}
			t.Author = usr
			ID := bleveTopicID(&t,nil)
			err := bleveIndex.Index(ID, t)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		replies := new(models.Replies)
		if err := tx.All(replies); err != nil {
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
			r.Author = usr
			ID := bleveTopicID(t,&r)
			err := bleveIndex.Index(ID, r)
			if err != nil {
				return errors.WithStack(err)
			}
		}
		return nil
	})
}

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
	if err := tx.Find(cat,topic.CategoryID); err != nil {
		c.Flash().Add("danger", T.Translate(c, "category-not-found"))
		return c.Error(404, err)
	}
	forum := &models.Forum{}
	if err := tx.Find(forum,cat.ParentCategory); err != nil {
		c.Flash().Add("danger", T.Translate(c, "forum-not-found"))
		return c.Error(404, err)
	}
	return c.Redirect(302, "topicGetPath()", render.Data{"forum_title": forum.Title, "cat_title": cat.Title, "tid": topic.ID})
}

func Search(c buffalo.Context) error {
	if c.Param("query") == "" {
		return c.Render(200, r.HTML("/search/search.plush.html"))
	}

	// query string is human syntax, see: http://blevesearch.com/docs/Query-String-Query/
	query := bleve.NewQueryStringQuery(c.Param("query"))
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

// just a random separator
const schSS = " __\000-__ "

func bleveTopicID(t *models.Topic,r *models.Reply) string {
	if r != nil {
		return strings.Join([]string{t.Title, r.TopicID.String(), DisplayName(r.Author),r.ID.String()}, schSS)
	}
	return strings.Join([]string{t.Title, t.ID.String(), DisplayName(t.Author)}, schSS)
}

func bleveTopicFromID(s string) (*models.Topic) {
	tsli := strings.Split(s, schSS)
	ID, _ := uuid.FromString(tsli[1])
	t := models.Topic{
		ID:         ID,
		Title:       tsli[0],
		Author:      &models.User{Nick: tsli[2]},
	}
	if len(tsli) > 3 { // we have a reply on our hands
		rID, _ := uuid.FromString(tsli[3])
		t.Replies = []models.Reply{{ID: rID}}
	}
	return &t
}
