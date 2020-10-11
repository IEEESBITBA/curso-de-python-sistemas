// Copyright 2018 The go-saloon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mailers

import (
	"fmt"
	"path/filepath"

	"github.com/IEEESBITBA/Curso-de-Python-Sistemas/models"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/mail"
	"github.com/pkg/errors"
)

/*
List-Post: <mailto:reply+00105748c619555d4a6c80b4faccec22003b863b33e73ae092cf0000000116c2ac9c92a169ce1238ebbe@reply.github.com>
List-Unsubscribe: <mailto:unsub+00105748c619555d4a6c80b4faccec22003b863b33e73ae092cf0000000116c2ac9c92a169ce1238ebbe@reply.github.com>, <https://github.com/notifications/unsubscribe/ABBXSLhgVLtfNtdMGG1Y0aRw9bFiNJc_ks5teuIcgaJpZM4Ss4xE>
*/

// NewTopicNotify Sends an email out to users about a new topic on their subscribed category
// Subscription should be checked beforehand
func NewTopicNotify(c buffalo.Context, topic *models.Topic, recpts []models.User) error {

	m := mail.NewMessage()
	m.SetHeader("Reply-To", notify.ReplyTo)
	m.SetHeader("Message-ID", fmt.Sprintf("<topic/%s@%s>", topic.ID, notify.MessageID))
	m.SetHeader("List-ID", notify.ListID)
	m.SetHeader("List-Archive", notify.ListArchive)
	m.SetHeader("List-Unsubscribe", notify.ListUnsubscribe)
	m.SetHeader("X-Auto-Response-Suppress", "All")

	m.Subject = notify.SubjectHdr + " " + topic.Title
	m.From = fmt.Sprintf("%s <%s>", topic.Author.Name, notify.From)
	m.To = nil
	m.Bcc = nil
	for _, usr := range recpts {
		m.Bcc = append(m.Bcc, usr.Email)
	}

	data := map[string]interface{}{
		"content":     topic.Content,
		"unsubscribe": notify.ListUnsubscribe,
		"visit":       notify.ListArchive + "/topics/detail/" + topic.ID.String(),
	}

	err := m.AddBodies(
		data,
		//r.Plain("mail/notify.txt"),
		r.HTML("mail/notify.html"),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	err = smtp.Send(m)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// NewReplyNotify Sends an email out to users about a reply on their subscribed topic
// Subscription should be checked beforehand
func NewReplyNotify(c buffalo.Context, topic *models.Topic, reply *models.Reply, recpts []models.User) error {
	m := mail.NewMessage()
	forumTitle := c.Param("forum_title")
	catTitle := c.Param("cat_title")
	topicPath := fmt.Sprintf("f/%s/c/%s/%s", forumTitle, catTitle, topic.ID)
	unsubscribePath := "u"
	m.SetHeader("Reply-To", notify.ReplyTo) //http://site.com/f/Curselli/c/Clases-1/ad2f50ae-11bd-4fea-aed2-69d511225edc/
	m.SetHeader("Message-ID", fmt.Sprintf("<f/%s/c/%s/%s@%s>", forumTitle, catTitle, reply.ID, notify.MessageID))
	m.SetHeader("In-Reply-To", fmt.Sprintf("<%s@%s>", topicPath, notify.InReplyTo))
	m.SetHeader("List-ID", notify.ListID)
	m.SetHeader("List-Archive", notify.ListArchive)
	m.SetHeader("List-Unsubscribe", notify.ListUnsubscribe)
	m.SetHeader("X-Auto-Response-Suppress", "All")

	m.Subject = notify.SubjectHdr + ": " + topic.Title

	m.From = fmt.Sprintf("%s <%s>", displayName(reply.Author), notify.From)
	m.To = nil
	m.Bcc = nil
	for _, usr := range recpts {
		m.Bcc = append(m.Bcc, usr.Email)
	}

	data := map[string]interface{}{
		"content":     reply.Content,
		"unsubscribe": filepath.Join(notify.ListArchive, unsubscribePath),
		"visit":       filepath.Join(notify.ListArchive, topicPath),
	}
	//
	err := m.AddBodies(
		data,
		//r.Plain("mail/notify.txt"),
		r.HTML("mail/notify.plush.html"),
	)
	if err != nil {
		return errors.WithStack(err)
	}
	c.Logger().Debugf("SEND %v", m)
	go func() { // run mailer asynchronously so process does not hang
		if err := smtp.Send(m); err != nil {
			c.Logger().Errorf("Failed sending notification messages for reply %s: %s", reply.ID, err)
		} else {
			c.Logger().Debugf("Success sending notification messages for reply %s", reply.ID)
		}
	}()
	return nil
}

// NewEvaluationSuccessNotify  Notifies a user/s of their success in passing an evaluation
func NewEvaluationSuccessNotify(c buffalo.Context, eval *models.Evaluation, recpts []models.User) error {
	m := mail.NewMessage()
	user := c.Value("current_user").(*models.User)
	if user.Subscribed(eval.ID) { // We subscribe the user once the email is successfully sent
		return nil
	}
	evalPath := fmt.Sprintf("curso-python/eval/e/%s", eval.ID)
	m.SetHeader("Reply-To", notify.ReplyTo) //http://site.com/f/Curselli/c/Clases-1/ad2f50ae-11bd-4fea-aed2-69d511225edc/
	m.SetHeader("Message-ID", fmt.Sprintf("<%s/%s@%s>", evalPath, user.ID, notify.MessageID))
	m.SetHeader("In-Reply-To", fmt.Sprintf("<%s@%s>", evalPath, notify.InReplyTo))
	m.SetHeader("List-ID", notify.ListID)
	m.SetHeader("List-Archive", notify.ListArchive)
	m.SetHeader("List-Unsubscribe", notify.ListUnsubscribe)
	m.SetHeader("X-Auto-Response-Suppress", "All")

	m.Subject = "Desafío aprobado: " + eval.Title

	m.From = fmt.Sprintf("%s <%s>", "Curso Python", notify.From)
	m.To = nil
	m.Bcc = nil
	for _, usr := range recpts {
		m.Bcc = append(m.Bcc, usr.Email)
	}

	data := map[string]interface{}{
		"content":     "No responder a este mensaje.",
		"unsubscribe": filepath.Join(notify.ListArchive),
		"visit":       filepath.Join(notify.ListArchive, evalPath),
	}
	//
	err := m.AddBodies(
		data,
		//r.Plain("mail/notify.txt"),
		r.HTML("mail/notify.plush.html"),
	)
	if err != nil {
		return errors.WithStack(err)
	}
	c.Logger().Printf("SEND %v", m)
	err = smtp.Send(m)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func displayName(u interface{}) string {
	user, ok := u.(*models.User)
	if !ok {
		userCopy := u.(models.User)
		user = &userCopy
	}
	if user.Nick != "" {
		return user.Nick
	}
	return user.Name
}
