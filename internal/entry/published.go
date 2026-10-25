package entry

import "time"

type Published struct {
	ID          string
	Title       *string
	ContentHTML HTML
	HasDataJSON bool
	CreatedAt   time.Time
}
