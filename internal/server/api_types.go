package server

import (
	"time"

	"github.com/ananthakumaran/paisa/internal/config"
)

// ConfigResponse is the JSON body returned by GET /api/config.
type ConfigResponse struct {
	Config   config.Config `json:"config"`
	Accounts []string      `json:"accounts"`
	Now      *time.Time    `json:"now"`
	Schema   any           `json:"schema"`
}
