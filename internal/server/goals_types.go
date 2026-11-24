package server

import "github.com/ananthakumaran/paisa/internal/server/goal"

// GoalsResponse is the JSON body returned by GET /api/goals.
type GoalsResponse struct {
	Goals []goal.GoalSummary `json:"goals"`
}
