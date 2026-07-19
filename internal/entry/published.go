package entry

import "time"

type Published struct {
	ID          string
	Title       *string
	ContentHTML HTML
	CreatedAt   time.Time
}
