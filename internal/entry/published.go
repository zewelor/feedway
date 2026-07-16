package entry

import "time"

type Published struct {
	ID          string
	Title       *string
	ContentHTML string
	CreatedAt   time.Time
}
