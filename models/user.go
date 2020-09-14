package models

import (
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/gobuffalo/flect"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/pop/v5/slices"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// User is used by pop to map your users database table to your go code.
type User struct {
	ID            uuid.UUID   `json:"id" db:"id"`
	Name          string      `json:"name" db:"name"`
	Nick          string      `json:"nick" db:"nick" form:"nick"`
	Provider      string      `json:"provider" db:"provider"`
	ProviderID    string      `json:"provider_id" db:"provider_id"`
	Email         string      `json:"email" db:"email"`
	Role          string      `json:"role" db:"role"`
	AvatarURL     string      `json:"avatar" db:"avatar_url"`
	Subscriptions slices.UUID `json:"subscriptions" db:"subscriptions"`
	CreatedAt     time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at" db:"updated_at"`

	Theme string `db:"-" json:"-" form:"code-theme"`
}

// String is not required by pop and may be deleted
func (u User) String() string {
	ju, _ := json.Marshal(u)
	return string(ju)
}

func (u User) Icon(label string) template.HTML {
	var icon string
	switch u.Role {
	case "admin":
		icon = "shield"
	case "banned":
		icon = "ban-circle"
	default:
		icon = "user"
	}
	return template.HTML(fmt.Sprintf("<i title=\"%s\" class=\"icon-%s\"> </i>%s",u.Role, icon, flect.Capitalize(label)))
}

// for now just return u.AvatarURL
func (u User) ImageSrc() string {
	return u.AvatarURL
}

func (u User) IsAuthor(id uuid.UUID) bool {
	return u.ID.String() == id.String()
}

// Users is not required by pop and may be deleted
type Users []User

// String is not required by pop and may be deleted
func (u Users) String() string {
	ju, _ := json.Marshal(u)
	return string(ju)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (u *User) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: u.Name, Name: "Name"},
		//&validators.StringIsPresent{Field: u.Nick, Name: "Nick"},
		&validators.StringIsPresent{Field: u.Provider, Name: "Provider"},
		&validators.StringIsPresent{Field: u.ProviderID, Name: "ProviderID"},
		&validators.StringIsPresent{Field: u.Email, Name: "Email"},
		//&validators.StringIsPresent{Field: u.Role, Name: "Role"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (u *User) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (u *User) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
