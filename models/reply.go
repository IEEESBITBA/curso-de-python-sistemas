package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Reply is used by pop to map your replies database table to your go code.
type Reply struct {
	ID        uuid.UUID `json:"id" db:"id"`
	AuthorID  uuid.UUID `json:"author_id" db:"author_id"`
	TopicID   uuid.UUID `json:"topic_id" db:"topic_id"`
	Content   string    `json:"content" db:"content" form:"content"`
	Deleted   bool      `json:"deleted" db:"deleted"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	Author *User  `json:"-" db:"-"`
	Topic  *Topic `json:"-" db:"-"`
}

// String is not required by pop and may be deleted
func (r Reply) String() string {
	jr, _ := json.Marshal(r)
	return string(jr)
}

// Replies and sort algorithm
type Replies []Reply

func (r Replies) Len() int           { return len(r) }
func (r Replies) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r Replies) Less(i, j int) bool { return r[i].CreatedAt.Before(r[j].CreatedAt) }

// String is not required by pop and may be deleted
func (r Replies) String() string {
	jr, _ := json.Marshal(r)
	return string(jr)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (r *Reply) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: r.Content, Name: "Content"},
	), nil
}
