package models

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/pop/v5/slices"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Forum is used by pop to map your forums database table to your go code.
type Forum struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	Title       string      `json:"title" db:"title" form:"title"`
	Description string      `json:"description" db:"description" form:"description"`
	Logo        []byte      `json:"logo" db:"logo" form:"logo"`
	Defcon      string      `json:"defcon" db:"defcon" form:"access"` // level of access needed to see forum
	Staff       slices.UUID `json:"staff" db:"staff" form:"staff"`    // moderator IDs
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`

	StaffEmails string `form:"staffemail" db:"-"`
}

// String is not required by pop and may be deleted
func (f Forum) String() string {
	jf, _ := json.Marshal(f)
	return string(jf)
}

func (f Forum) LogoImage() string {
	return base64.StdEncoding.EncodeToString(f.Logo)
}

// Forums is not required by pop and may be deleted
type Forums []Forum

// String is not required by pop and may be deleted
func (f Forums) String() string {
	jf, _ := json.Marshal(f)
	return string(jf)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (f *Forum) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: f.Title, Name: "Title"},
		&validators.StringIsPresent{Field: f.Description, Name: "Description"},
		//&validators.StringIsPresent{Field: f.Defcon, Name: "Defcon"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (f *Forum) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (f *Forum) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
