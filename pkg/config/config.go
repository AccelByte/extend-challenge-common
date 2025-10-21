package config

import "github.com/AccelByte/extend-challenge-common/pkg/domain"

// Config represents the top-level configuration loaded from challenges.json.
// This structure is parsed from JSON and validated during application startup.
type Config struct {
	Challenges []*domain.Challenge `json:"challenges"`
}
