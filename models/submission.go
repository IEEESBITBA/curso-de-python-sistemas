package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Submission is used by pop to map your submissions database table to your go code.
// Submission is basically a google forms response or actual form template mockup
type Submission struct {
	ID                 uuid.UUID `json:"id" db:"id"`
	ForumID            uuid.UUID `json:"forum_id" db:"forum_id"`
	IsTemplate         bool      `json:"is_template" db:"is_template"`
	UserID             uuid.UUID `json:"user_id" db:"user_id"`
	RequireEmailVerify bool      `json:"require_email_verify" db:"require_email_verify"`
	// Template fields (isTemplate == true)
	Title       nulls.String `form:"title" json:"title,omitempty" db:"title"`
	Description nulls.String `form:"description" json:"description,omitempty" db:"description"`
	Schemas     nulls.String `form:"schemas" json:"schemas,omitempty" db:"schemas"`
	Hidden      bool         `form:"hidden" json:"hidden" db:"hidden"`
	Deleted     bool         `json:"deleted" db:"deleted"`
	Editable    bool         `form:"editable" json:"editable" db:"editable"`
	Anonymous   bool         `form:"anonymous" json:"anonymous" db:"anonymous"`
	// Response fields (isTemplate == false)
	SubmissionID  uuid.NullUUID   `json:"submission_id,omitempty" db:"submission_id"`
	Response      nulls.String    `json:"response,omitempty" db:"response"`
	HasAttachment bool            `json:"has_attachment" db:"has_attachment"`
	Zip           nulls.ByteSlice `json:"-" db:"zip"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
	// Non-DB fields
	User *User `json:"-" db:"-"`
}

// String is not required by pop and may be deleted
func (s Submission) String() string {
	js, _ := json.Marshal(s)
	return string(js)
}

// Template creates a submission populated with template data
// The resulting submission is a user response and not a template!
func (s *Submission) Template(user *User) *Submission {
	t := new(Submission)
	t.Editable = s.Editable
	t.ForumID = s.ForumID
	t.SubmissionID.UUID, t.SubmissionID.Valid = s.ID, true
	t.IsTemplate = false
	t.Anonymous = s.Anonymous
	if !s.Anonymous {
		t.UserID = user.ID
	}
	return t
}

// Submissions is not required by pop and may be deleted
type Submissions []Submission

// String is not required by pop and may be deleted
func (s Submissions) String() string {
	js, _ := json.Marshal(s)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
func (s *Submission) Validate(tx *pop.Connection) (*validate.Errors, error) {
	v := []validate.Validator{&validators.UUIDIsPresent{Field: s.ForumID, Name: "ForumID"}}
	v = append(v, &validators.UUIDIsPresent{Field: s.UserID, Name: "UserID"})

	if s.IsTemplate {
		v = append(v, &validators.StringIsPresent{Field: s.Schemas.String, Name: "Schemas"},
			&validators.StringIsPresent{Field: s.Description.String, Name: "Description"},
			&validators.StringIsPresent{Field: s.Title.String, Name: "Title"},
		)
		if s.HasAttachment {
			v = append(v, &validators.BytesArePresent{Field: s.Zip.ByteSlice, Message: "Attachment not found", Name: "Zip"})
		}
		return validate.Validate(v...), nil
	} else {
		v = append(v, &validators.StringIsPresent{Field: s.Response.String, Name: "Response"},
			&validators.UUIDIsPresent{Field: s.SubmissionID.UUID, Name: "SubmissionID"})
		return validate.Validate(v...), nil
	}
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (s *Submission) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (s *Submission) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
