// Package history compacts unchanged same-day measurements without losing retries.
package history

import (
	"bytes"
	"encoding/json"
	"time"
)

// Earlier orders a payload by source time so A -> B -> A remains three changes.
func Earlier(left, right json.RawMessage) bool {
	leftTime, rightTime := observedTime(left), observedTime(right)
	if leftTime.Equal(rightTime) {
		return bytes.Compare(left, right) < 0
	}
	return leftTime.Before(rightTime)
}

func observedTime(raw json.RawMessage) time.Time {
	var observation struct {
		ObservedAt time.Time `json:"observed_at"`
	}
	if err := json.Unmarshal(raw, &observation); err != nil {
		return time.Time{} // Normal observation decoding rejects invalid data before writes commit.
	}
	return observation.ObservedAt
}
