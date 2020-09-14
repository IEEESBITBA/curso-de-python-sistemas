package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/pop/v5/slices"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Topic is used by pop to map your topics database table to your go code.
type Topic struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	Title       string      `json:"title" db:"title" form:"title"`
	Content     string      `json:"content" db:"content" form:"content"`
	AuthorID    uuid.UUID   `json:"author_id" db:"author_id"`
	CategoryID  uuid.UUID   `json:"category_id" db:"category_id" `
	Deleted     bool        `json:"deleted" db:"deleted"`
	Subscribers slices.UUID `json:"subscribers" db:"subscribers"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`

	Author   *User     `json:"-" db:"-"`
	Category *Category `json:"-" db:"-"`
	Replies  Replies   `json:"-" db:"-"`
}

func (t Topic) Authors() Users {
	var set = make(map[uuid.UUID]User, 1+len(t.Replies))
	set[t.Author.ID] = *t.Author
	for _, reply := range t.Replies {
		_, dup := set[reply.AuthorID]
		if dup {
			continue
		}
		if reply.Author != nil {
			set[reply.AuthorID] = *reply.Author
		}
	}
	authors := make([]User, 0, len(set))
	for _, v := range set {
		authors = append(authors, v)
	}
	return Users(authors)
}

// String is not required by pop and may be deleted
func (t Topic) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

func (t Topic) LastUpdate() time.Time {
	last := func(a, b time.Time) time.Time {
		if a.UTC().After(b.UTC()) {
			return a.UTC()
		}
		return b.UTC()
	}
	v := last(t.CreatedAt, t.UpdatedAt)
	for _, reply := range t.Replies {
		v = last(v, reply.CreatedAt)
		v = last(v, reply.UpdatedAt)
	}
	return v
}

func (t Topic) Subscribed(id uuid.UUID) bool {
	for _, usr := range t.Subscribers {
		if usr == id {
			return true
		}
	}
	return false
}

func (t *Topic) AddSubscriber(id uuid.UUID) {
	set := make(map[uuid.UUID]struct{})
	set[id] = struct{}{}
	for _, sub := range t.Subscribers {
		set[sub] = struct{}{}
	}
	subs := make(slices.UUID, 0, len(set))
	for sub := range set {
		subs = append(subs, sub)
	}
	t.Subscribers = subs
}

func (t *Topic) RemoveSubscriber(id uuid.UUID) {
	set := make(map[uuid.UUID]struct{})
	for _, sub := range t.Subscribers {
		if sub != id {
			set[sub] = struct{}{}
		}
	}
	subs := make(slices.UUID, 0, len(set))
	for sub := range set {
		subs = append(subs, sub)
	}
	t.Subscribers = subs
}

// Topics is not required by pop and may be deleted

type Topics []Topic

func (t Topics) Len() int           { return len(t) }
func (t Topics) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t Topics) Less(i, j int) bool { return t[i].CreatedAt.After(t[j].CreatedAt) }

// String is not required by pop and may be deleted
func (t Topics) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (t *Topic) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: t.Title, Name: "Title"},
		&validators.StringIsPresent{Field: t.Content, Name: "Content"},
	), nil
}
