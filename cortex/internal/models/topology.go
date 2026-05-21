package models

import (
	"time"
)

type ServiceAlias struct {
	ID        string     `json:"id" db:"id"`
	Alias     string     `json:"alias" db:"alias"`
	Canonical string    `json:"canonical" db:"canonical"`
	ValidFrom time.Time `json:"valid_from" db:"valid_from"`
	ValidTo   *time.Time `json:"valid_to,omitempty" db:"valid_to"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}