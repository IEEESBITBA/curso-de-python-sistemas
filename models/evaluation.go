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

// Evaluation is used by pop to map your evaluations database table to your go code.
type Evaluation struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	Title       string       `json:"title" db:"title" form:"title"`
	Description string       `json:"description" db:"description" form:"description"`
	Content     string       `json:"content" db:"content" form:"content"`
	Solution    string       `json:"solution" db:"solution" form:"solution"`
	Hidden      bool         `json:"hidden" db:"hidden" form:"hidden"`
	Deleted     bool         `json:"deleted" db:"deleted" form:"deleted"`
	Inputs      nulls.String `json:"inputs" db:"inputs" form:"stdin"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (e Evaluation) String() string {
	je, _ := json.Marshal(e)
	return string(je)
}

// Evaluations is not required by pop and may be deleted
type Evaluations []Evaluation

func (e Evaluations) Len() int      { return len(e) }
func (e Evaluations) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
func (e Evaluations) Less(i, j int) bool {
	// Un branchless algorithm para que ande mas rapido
	return ((e[i].Hidden == e[j].Hidden) && e[i].CreatedAt.Before(e[j].CreatedAt)) ||
		((e[i].Hidden != e[j].Hidden) && (e[j].Hidden))
}

// String is not required by pop and may be deleted
func (e Evaluations) String() string {
	je, _ := json.Marshal(e)
	return string(je)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (e *Evaluation) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: e.Title, Name: "Title"},
		&validators.StringIsPresent{Field: e.Description, Name: "Description"},
		&validators.StringIsPresent{Field: e.Content, Name: "Content"},
		&validators.StringIsPresent{Field: e.Solution, Name: "Solution"},
	), nil
}

//// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
//// This method is not required and may be deleted.
//func (e *Evaluation) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
//	return validate.NewErrors(), nil
//}
//
//// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
//// This method is not required and may be deleted.
//func (e *Evaluation) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
//	return validate.NewErrors(), nil
//}
