package saas

import (
	"errors"
	"github.com/google/uuid"
	"net/mail"
	"strings"
	"unicode/utf8"
)

type TrialInput struct {
	Name      string    `json:"name"`
	OwnerName string    `json:"owner_name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	PlanID    uuid.UUID `json:"plan_id"`
}

func (i *TrialInput) Validate() error {
	i.Name = strings.TrimSpace(i.Name)
	i.OwnerName = strings.TrimSpace(i.OwnerName)
	i.Email = strings.ToLower(strings.TrimSpace(i.Email))
	a, err := mail.ParseAddress(i.Email)
	if err != nil || a.Address != i.Email || len(i.Email) > 255 || i.PlanID == uuid.Nil ||
		utf8.RuneCountInString(i.Name) < 1 || utf8.RuneCountInString(i.Name) > 120 ||
		utf8.RuneCountInString(i.OwnerName) < 1 || utf8.RuneCountInString(i.OwnerName) > 120 ||
		utf8.RuneCountInString(i.Password) < 12 || len(i.Password) > 72 {
		return errors.New("saas.invalid_trial_signup")
	}
	return nil
}
