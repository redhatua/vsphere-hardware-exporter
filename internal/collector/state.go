package collector

import "time"

// State describes the outcome of the most recent refresh.
type State struct {
	Up          bool
	LastSuccess time.Time
	Duration    time.Duration
}
