package actions

import (
	"sort"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
)

func TopicGet(c buffalo.Context) error {
	f := c.Value("forum").(*models.Forum)
	tid := c.Param("tid")
	renderData := render.Data{"forum_title": f.Title, "cat_title": c.Param("cat_title"), "tid": tid}
	topic, err := loadTopic(c, tid)
	if err != nil {
		return c.Redirect(302, "catPath()", renderData)
	}
	c.Set("topic", topic)
	return c.Render(200, r.HTML("topics/get.plush.html"))

	//return c.Redirect(200,"topicGetPath()", renderData)//c.Render(200,r.HTML("topics/index.plush.html"))
}

func TopicCreateGet(c buffalo.Context) error {
	return c.Render(200, r.HTML("topics/create.plush.html"))
}

func TopicCreatePost(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)
	topic := &models.Topic{}
	if err := c.Bind(topic); err != nil {
		return errors.WithStack(err)
	}
	topic.Author = c.Value("current_user").(*models.User)
	cat := new(models.Category)
	q := tx.Where("title = ?", c.Param("cat_title"))
	err := q.First(cat)
	if err != nil {
		c.Flash().Add("danger", "Error while seeking category")
		return c.Redirect(302, "forumPath()")
	}

	topic.Category = cat
	topic.AuthorID = topic.Author.ID
	topic.CategoryID = topic.Category.ID
	topic.AddSubscriber(topic.AuthorID)
	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(topic)
	if err != nil {
		return errors.WithStack(err)
	}
	if verrs.HasAny() {
		c.Set("topic", topic)
		c.Set("errors", verrs.Errors)
		return c.Render(422, r.HTML("topics/create.plush.html"))
	}
	//err = newTopicNotify(c, topic)
	//if err != nil {
	//	return errors.WithStack(err)
	//}
	_ = tx.UpdateColumns(cat, "updated_at")
	f := c.Value("forum").(*models.Forum)
	c.Logger().Infof("TopicCreatePost finish: %s, by %s",topic.Title,topic.Author.Email)
	c.Flash().Add("success", T.Translate(c, "topic-add-success"))
	return c.Redirect(302, "catPath()", render.Data{"forum_title": f.Title, "cat_title": cat.Title})
}

func TopicEditGet(c buffalo.Context) error {
	return c.Render(200, r.HTML("topics/create.plush.html"))
}

func TopicEditPost(c buffalo.Context) error {
	topic := c.Value("topic").(*models.Topic)
	tx := c.Value("tx").(*pop.Connection)
	if err := c.Bind(topic); err != nil {
		return errors.WithStack(err)
	}
	if err := tx.Update(topic); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "topic-edit-success"))
	f := c.Value("forum").(*models.Forum)
	return c.Redirect(302, "topicGetPath()", render.Data{"forum_title": f.Title,
		"cat_title": c.Param("cat_title"), "tid": c.Param("tid")})
}

func SetCurrentTopic(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		//topic := &models.Topic{}
		topic, err := loadTopic(c, c.Param("tid"))
		if err != nil {
			c.Flash().Add("danger", T.Translate(c, "topic-not-found"))
			return c.Error(404,err)
		}
		c.Set("topic", topic)
		return next(c)
	}
}

func loadTopic(c buffalo.Context, tid string) (*models.Topic, error) {
	tx := c.Value("tx").(*pop.Connection)
	topic := &models.Topic{}
	if err := c.Bind(topic); err != nil {
		c.Logger().Errorf("'c.Bind(topic)' FAILED!", topic)
		return nil, errors.WithStack(err)
	}
	if err := tx.Find(topic, tid); err != nil {
		c.Logger().Errorf("'tx.Find(%s, %s)' FAILED!", topic, tid)
		return nil, c.Error(404, err)
	}
	cat := new(models.Category)
	if err := tx.Find(cat, topic.CategoryID); err != nil {
		c.Logger().Errorf("'tx.Find(cat, %s)' FAILED!", topic.CategoryID)
		return nil, c.Error(404, err)
	}
	usr := new(models.User)
	if err := tx.Find(usr, topic.AuthorID); err != nil {
		c.Logger().Errorf("'tx.Find(usr, %s)' FAILED!", topic.AuthorID)
		return nil, c.Error(404, err)
	}
	if err := tx.BelongsTo(topic).All(&topic.Replies); err != nil {
		c.Logger().Errorf("'tx.BelongsTo(%s).All(&topic.Replies)' FAILED!", topic)
		return nil, c.Error(404, err)
	}
	topic.Category = cat
	topic.Author = usr
	replies := make(models.Replies, 0, len(topic.Replies))
	for i := range topic.Replies {
		reply, err := loadReply(c, topic.Replies[i].ID.String())
		if err != nil {
			c.Logger().Errorf("'loadReply(c, %s)' FAILED!", topic.Replies[i].ID.String())
			return nil, c.Error(404, err)
		}
		if reply.Deleted {
			continue
		}
		replies = append(replies, *reply)
	}
	topic.Replies = replies
	sort.Sort(topic.Replies)
	return topic, nil
}
