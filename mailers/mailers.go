package mailers

import (
"bytes"
"encoding/binary"
"encoding/gob"
"io"
"log"
"net"
	"path/filepath"
	"text/template"

"github.com/gobuffalo/buffalo/mail"
"github.com/gobuffalo/buffalo/render"
"github.com/gobuffalo/envy"
"github.com/gobuffalo/packr"
"github.com/pkg/errors"
)

var smtp mail.Sender
var r *render.Engine

var notify struct {
	// A Reply-To address is identified by inserting the Reply-To header in your email.
	//It is the email address that the reply message is sent when you want the reply to go to an email address that is different than the From: address
	ReplyTo         string
	//Message-ID is a unique identifier for a digital message, most commonly a globally unique identifier used in email
	// This particular field usually has the form 'example.com' (what comes after the '@')
	MessageID       string
	// The value of an In-Reply-To field is tokenizable, consisting of a series of words and message identifiers.
	//According to 822, In-Reply-To lists parents, and References lists ``other correspondence.'' Some MUAs do in fact put parents into In-Reply-To. However, very few readers are able to parse the complicated syntax of In-Reply-To specified by 822, let alone the syntactically incorrect fields that show up in practice:
	// tokenizable, see: https://cr.yp.to/immhf/token.html
	InReplyTo       string
	// https://tools.ietf.org/html/rfc2919 The syntax for a list identifier in ABNF [RFC2234] follows:
	//   list-id = optional-label <list-label "." list-id-namespace>
	// i.e: List-Id: List Header Mailing List <list-header.nisto.com>
	ListID          string
	// We use ListArchive to save our website: https://curso.whittileaks.com
	ListArchive     string
	// https://www.ietf.org/rfc/rfc2369.txt
	// can have this form: <https://github.com/notifications/unsubscribe/ABBXSLhgVLtfNtdMGG1Y0aRw9bFiNJc_ks5teuIcgaJpZM4Ss4xE>
	ListUnsubscribe string
	// The name of the mail as shown to the user
	SubjectHdr      string
	// Who sent the mail?
	From            string
}

func init() {

	// Pulling config from the env.
	send := envy.Get("CURSO_SEND_MAIL", "")
	port := envy.Get("SMTP_PORT", "")
	host := envy.Get("SMTP_HOST", "")
	user := envy.Get("SMTP_USER", "")
	password := envy.Get("SMTP_PASSWORD", "")

	notify.ReplyTo = envy.Get("CURSO_MAIL_NOTIFY_REPLY_TO", "")
	notify.MessageID = envy.Get("CURSO_MAIL_NOTIFY_MESSAGE_ID", "")
	notify.InReplyTo = envy.Get("CURSO_MAIL_NOTIFY_IN_REPLY_TO", "")
	notify.ListID = envy.Get("CURSO_MAIL_NOTIFY_LIST_ID", "")
	notify.ListArchive = envy.Get("CURSO_MAIL_NOTIFY_LIST_ARCHIVE", envy.Get("FORUM_HOST",""))
	notify.ListUnsubscribe = filepath.Join(notify.ListArchive, "u")
	notify.SubjectHdr = envy.Get("CURSO_MAIL_NOTIFY_SUBJECT_HDR", "")
	notify.From = envy.Get("CURSO_MAIL_NOTIFY_FROM", "")

	var err error

	switch send {
	case "1", "y", "yes", "Y","true":
		smtp, err = mail.NewSMTPSender(host, port, user, password)
	case "remote":
		smtp, err = newRelaySender(host, port, user, password)
	default:
		smtp = noMailSender{}
	}
	if err != nil {
		log.Fatal(err)
	}

	r = render.New(render.Options{
		HTMLLayout:   "mail/layout.html",
		TemplatesBox: packr.NewBox("../templates"),
		Helpers:      render.Helpers{},
		TemplateEngines: map[string]render.TemplateEngine{
			"txt": PlainTextTemplateEngine,
		},
	})

}

func PlainTextTemplateEngine(input string, data map[string]interface{}, helpers map[string]interface{}) (string, error) {
	// since go templates don't have the concept of an optional map argument like Plush does
	// add this "null" map so it can be used in templates like this:
	// {{ partial "flash.html" .nilOpts }}
	data["nilOpts"] = map[string]interface{}{}

	t := template.New(input)
	if helpers != nil {
		t = t.Funcs(helpers)
	}

	t, err := t.Parse(input)
	if err != nil {
		return "", err
	}

	bb := &bytes.Buffer{}
	err = t.Execute(bb, data)
	return bb.String(), err
}

// noMailSender is a mail.Sender with no actual delivery.
type noMailSender struct{}

func (noMailSender) Send(mail.Message) error {
	return nil
}

type relaySender struct {
	addr string
}

func newRelaySender(host, port, user, password string) (*relaySender, error) {
	return &relaySender{host + ":" + port}, nil
}

func (rs relaySender) Send(m mail.Message) error {
	conn, err := net.Dial("tcp", rs.addr)
	if err != nil {
		return errors.WithStack(err)
	}
	defer conn.Close()

	o := new(bytes.Buffer)
	for i := range m.Attachments {
		att := &m.Attachments[i]
		r := new(bytes.Buffer)
		_, err := io.Copy(r, att.Reader)
		if err != nil {
			return errors.WithStack(err)
		}
		att.Reader = r
	}
	m.Context = nil
	err = gob.NewEncoder(o).Encode(m)
	if err != nil {
		return errors.WithStack(err)
	}

	var hdr [8]byte
	n := int64(o.Len())
	binary.BigEndian.PutUint64(hdr[:], uint64(n))
	_, err = conn.Write(hdr[:])
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = conn.Write(o.Bytes())
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}