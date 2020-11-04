package actions

import (
	"fmt"
	"sort"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/pop/v5"
	"github.com/pkg/errors"
)

// TopicGet render a topic. basis of the forum
func TopicGet(c buffalo.Context) error {
	tid := c.Param("tid")
	renderData := render.Data{"forum_title": c.Param("forum_title"), "cat_title": c.Param("cat_title"), "tid": tid}
	topic, err := loadTopic(c, tid)
	if err != nil {
		return c.Redirect(302, "catPath()", renderData)
	}
	c.Set("topic", topic)
	return c.Render(200, r.HTML("topics/get.plush.html"))
}

// TopicCreateGet renders the topic creation page
func TopicCreateGet(c buffalo.Context) error {
	return c.Render(200, r.HTML("topics/create.plush.html"))
}

// TopicCreatePost handles the topic creation event
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
	topic.AddVoter(topic.AuthorID)
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
	// Category topic add email notification
	//err = newTopicNotify(c, topic)
	//if err != nil {
	//	return errors.WithStack(err)
	//}
	u := c.Value("current_user").(*models.User)
	u.AddSubscription(topic.ID)
	_ = tx.UpdateColumns(u, "subscriptions")
	_ = tx.UpdateColumns(cat, "updated_at")
	f := c.Value("forum").(*models.Forum)
	c.Logger().Infof("TopicCreatePost finish: %s, by %s", topic.Title, topic.Author.Email)
	c.Flash().Add("success", T.Translate(c, "topic-add-success"))
	return c.Redirect(302, "catPath()", render.Data{"forum_title": f.Title, "cat_title": cat.Title})
}

// TopicDelete handles topic deletion event
func TopicDelete(c buffalo.Context) error {
	topic, err := loadTopic(c, c.Param("tid"))
	if err != nil {
		return errors.WithStack(err)
	}
	f := c.Value("forum").(*models.Forum)
	usr := c.Value("current_user").(*models.User)
	if !(usr.Role == "admin" || usr.ID == topic.AuthorID) {
		c.Flash().Add("danger", "You are not authorized to delete this topic")
		return c.Redirect(302, "topicGetPath()", render.Data{"forum_title": f.Title, "cat_title": c.Param("cat_title"),
			"tid": c.Param("tid")})
	}
	tx := c.Value("tx").(*pop.Connection)
	topic.Deleted = true
	if err := tx.UpdateColumns(topic, "deleted"); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", "Topic deleted successfully.")
	return c.Redirect(302, "catPath()", render.Data{"forum_title": f.Title, "cat_title": c.Param("cat_title")})
}

// TopicEditGet renders topic editing page. topic is already set by SetCurrentTopic
func TopicEditGet(c buffalo.Context) error {
	return c.Render(200, r.HTML("topics/create.plush.html"))
}

// TopicEditPost handles topic editing event
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
	return c.Redirect(302, "topicGetPath()", render.Data{"forum_title": c.Param("forum_title"),
		"cat_title": c.Param("cat_title"), "tid": c.Param("tid")})
}

func TopicArchivePost(c buffalo.Context) error {
	topic := c.Value("topic").(*models.Topic)
	u := c.Value("current_user").(*models.User)
	if topic.Author.ID != u.ID && u.Role != "admin" {
		return c.Redirect(403, "/")
	}
	tx := c.Value("tx").(*pop.Connection)
	topic.Archived = !topic.Archived
	if err := tx.UpdateColumns(topic, "archived"); err != nil {
		return errors.WithStack(err)
	}
	c.Flash().Add("success", T.Translate(c, "archived")+fmt.Sprintf(" %s: %t", topic.Title, topic.Archived))
	return c.Redirect(302, c.Request().Referer())
}

// SetCurrentTopic sets 'topic' in context for easy use in html template
func SetCurrentTopic(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		//topic := &models.Topic{}
		topic, err := loadTopic(c, c.Param("tid"))
		if err != nil || topic.Deleted {
			c.Flash().Add("danger", T.Translate(c, "topic-not-found"))
			return c.Error(404, err)
		}
		c.Set("topic", topic)
		return next(c)
	}
}

// TopicVote Handles voting event. adds vote to topic
func TopicVote(c buffalo.Context) error {
	topic := c.Value("topic").(*models.Topic)
	user := c.Value("current_user").(*models.User)
	if topic.Voted(user.ID) {
		c.Flash().Add("warning", "Ya votaste.")
	} else {
		topic.AddVoter(user.ID)
		tx := c.Value("tx").(*pop.Connection)
		if err := tx.UpdateColumns(topic, "voters"); err != nil {
			return c.Error(500, err)
		}
		c.Flash().Add("success", "Voto registrado.")
	}
	return c.Redirect(302, "catPath()", render.Data{"forum_title": c.Param("forum_title"),
		"cat_title": c.Param("cat_title")})
}

// TopicUnvote remove vote from topic
func TopicUnvote(c buffalo.Context) error {
	topic := c.Value("topic").(*models.Topic)
	user := c.Value("current_user").(*models.User)
	if topic.Voted(user.ID) {
		topic.RemoveVoter(user.ID)
		tx := c.Value("tx").(*pop.Connection)
		if err := tx.UpdateColumns(topic, "voters"); err != nil {
			return c.Error(500, err)
		}
	} else {
		c.Flash().Add("warning", "Error de voto.")
	}
	return c.Redirect(302, "catPath()", render.Data{"forum_title": c.Param("forum_title"),
		"cat_title": c.Param("cat_title")})
}

// loadTopic creates and populates a models.Topic from a uuid string
func loadTopic(c buffalo.Context, tid string) (*models.Topic, error) {
	tx := c.Value("tx").(*pop.Connection)
	topic := &models.Topic{}
	if err := tx.Find(topic, tid); err != nil {
		c.Logger().Errorf("'tx.Find(%q, %s)' FAILED!", topic, tid)
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
