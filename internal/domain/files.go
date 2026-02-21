package domain

import "time"

type File struct {
	Name         string     `db:"name"`
	Status       Status     `db:"status"`
	ErrorMessage string     `db:"error_message"`
	ProcessedAt  *time.Time `db:"processed_at"`
}
