package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/pop/v5/slices"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Category is used by pop to map your categories database table to your go code.
type Category struct {
	ID             uuid.UUID    `json:"id" db:"id"`
	Title          string       `json:"title" db:"title" form:"title"`
	Description    nulls.String `json:"description" db:"description" form:"description"`
	Subscribers    slices.UUID  `json:"subscribers" db:"subscribers"`
	ParentCategory nulls.UUID   `json:"parent_category" db:"parent_category"`
	CreatedAt      time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (c Category) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// AddSubscriber add a uuid to category.Subscribers
func (c *Category) AddSubscriber(id uuid.UUID) {
	set := make(map[uuid.UUID]struct{})
	set[id] = struct{}{}
	for _, sub := range c.Subscribers {
		set[sub] = struct{}{}
	}
	subs := make(slices.UUID, 0, len(set))
	for sub := range set {
		subs = append(subs, sub)
	}
	c.Subscribers = subs
}

// RemoveSubscriber remove a uuid from category.Subscribers
func (c *Category) RemoveSubscriber(id uuid.UUID) {
	set := make(map[uuid.UUID]struct{})
	for _, sub := range c.Subscribers {
		if sub != id {
			set[sub] = struct{}{}
		}
	}
	subs := make(slices.UUID, 0, len(set))
	for sub := range set {
		subs = append(subs, sub)
	}
	c.Subscribers = subs
}

// Categories is not required by pop and may be deleted
type Categories []Category

// String is not required by pop and may be deleted
func (c Categories) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

func (c Categories) Len() int      { return len(c) }
func (c Categories) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c Categories) Less(i, j int) bool {
	if c[i].Title == c[j].Title {
		return c[i].ID.String() < c[j].ID.String()
	}
	return c[i].Title < c[j].Title
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (c *Category) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: c.Title, Name: "Title"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (c *Category) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (c *Category) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
