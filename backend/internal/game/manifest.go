// backend/internal/game/manifest.go
package game

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Manifest is the per-handler configuration, loaded from a `manifest.yaml`
// embedded alongside the handler's Go source. It is the single source of
// truth for a game type's identity (slug, name, description, version),
// capabilities (solo support, supported payload versions), and tunable
// config bounds. On startup the manifest is upserted into the `game_types`
// table (see SyncGameTypes), and the API layer uses Bounds.ValidateAndFill
// to enforce min/max/default on every room config write.
//
// Changing a bound is a one-file edit: update the handler's manifest.yaml
// and restart the server. No DB migration is required.
type Manifest struct {
	Slug            string `yaml:"slug"`
	Name            string `yaml:"name"`
	Description     string `yaml:"description"`
	Version         string `yaml:"version"`
	SupportsSolo    bool   `yaml:"supports_solo"`
	PayloadVersions []int  `yaml:"payload_versions"`
	Config          Bounds `yaml:"config"`
}

// Bounds mirrors the flat JSONB shape we store in game_types.config and
// surface to the frontend via GET /api/game-types. Keeping manifest and DB
// schema structurally identical means the upsert is a single json.Marshal
// and the frontend consumes the same field names it always has.
//
// MaxPlayers is a pointer so a manifest may express "no cap" with null;
// in practice we recommend a concrete cap so the hub can reject joins
// before allocating per-player state.
type Bounds struct {
	MinRoundDurationSeconds      int  `yaml:"min_round_duration_seconds"      json:"min_round_duration_seconds"`
	MaxRoundDurationSeconds      int  `yaml:"max_round_duration_seconds"      json:"max_round_duration_seconds"`
	DefaultRoundDurationSeconds  int  `yaml:"default_round_duration_seconds"  json:"default_round_duration_seconds"`
	MinVotingDurationSeconds     int  `yaml:"min_voting_duration_seconds"     json:"min_voting_duration_seconds"`
	MaxVotingDurationSeconds     int  `yaml:"max_voting_duration_seconds"     json:"max_voting_duration_seconds"`
	DefaultVotingDurationSeconds int  `yaml:"default_voting_duration_seconds" json:"default_voting_duration_seconds"`
	MinRoundCount                int  `yaml:"min_round_count"                 json:"min_round_count"`
	MaxRoundCount                int  `yaml:"max_round_count"                 json:"max_round_count"`
	DefaultRoundCount            int  `yaml:"default_round_count"             json:"default_round_count"`
	MinPlayers                   int  `yaml:"min_players"                     json:"min_players"`
	MaxPlayers                   *int `yaml:"max_players"                     json:"max_players"`
}

// RoomConfig is the normalized per-room config written back to
// rooms.config. It is the authoritative shape the hub and the frontend
// both read; ValidateAndFill always emits exactly these fields.
type RoomConfig struct {
	RoundDurationSeconds  int  `json:"round_duration_seconds"`
	VotingDurationSeconds int  `json:"voting_duration_seconds"`
	RoundCount            int  `json:"round_count"`
	HostPaced             bool `json:"host_paced"`
	JokerCount            int  `json:"joker_count"`
	AllowSkipVote         bool `json:"allow_skip_vote"`
}

// LoadManifest parses a YAML manifest and fails fast on any internal
// inconsistency (e.g. default outside min/max, min > max). The same
// checks run for every handler at startup so a broken manifest refuses
// to boot rather than silently corrupting rooms later.
func LoadManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Slug == "" {
		return nil, fmt.Errorf("manifest: slug is required")
	}
	if m.Name == "" {
		return nil, fmt.Errorf("manifest %q: name is required", m.Slug)
	}
	if m.Version == "" {
		return nil, fmt.Errorf("manifest %q: version is required", m.Slug)
	}
	if len(m.PayloadVersions) == 0 {
		return nil, fmt.Errorf("manifest %q: payload_versions must list at least one version", m.Slug)
	}
	if err := m.Config.selfCheck(m.Slug); err != nil {
		return nil, err
	}
	return &m, nil
}

func (b Bounds) selfCheck(slug string) error {
	checks := []struct {
		field            string
		min, max, deflt  int
	}{
		{"round_duration_seconds", b.MinRoundDurationSeconds, b.MaxRoundDurationSeconds, b.DefaultRoundDurationSeconds},
		{"voting_duration_seconds", b.MinVotingDurationSeconds, b.MaxVotingDurationSeconds, b.DefaultVotingDurationSeconds},
		{"round_count", b.MinRoundCount, b.MaxRoundCount, b.DefaultRoundCount},
	}
	for _, c := range checks {
		if c.min < 0 || c.max <= 0 || c.min > c.max {
			return fmt.Errorf("manifest %q: %s bounds invalid (min=%d max=%d)", slug, c.field, c.min, c.max)
		}
		if c.deflt < c.min || c.deflt > c.max {
			return fmt.Errorf("manifest %q: %s default (%d) outside [%d,%d]", slug, c.field, c.deflt, c.min, c.max)
		}
	}
	if b.MinPlayers < 1 {
		return fmt.Errorf("manifest %q: min_players must be ≥ 1", slug)
	}
	if b.MaxPlayers != nil && *b.MaxPlayers < b.MinPlayers {
		return fmt.Errorf("manifest %q: max_players (%d) < min_players (%d)", slug, *b.MaxPlayers, b.MinPlayers)
	}
	return nil
}

// ConfigJSON returns the bounds serialized as JSON for the game_types.config
// JSONB column. We marshal through the JSON tags so the DB shape tracks
// exactly what the frontend expects from GET /api/game-types.
func (m *Manifest) ConfigJSON() (json.RawMessage, error) {
	return json.Marshal(m.Config)
}

// MaxPlayersOrDefault returns the handler's hard player cap, honoring the
// manifest's explicit max_players. A missing (nil) value means the
// operator did not set a cap; we return 0 which the hub treats as
// unbounded. This mirrors the contract documented on GameTypeHandler.MaxPlayers.
func (m *Manifest) MaxPlayersOrDefault() int {
	if m.Config.MaxPlayers == nil {
		return 0
	}
	return *m.Config.MaxPlayers
}

// ValidateAndFill enforces the manifest's bounds against a room config
// JSON blob. Missing fields are populated from the manifest defaults;
// out-of-range fields return a ValidationError so callers can render a
// precise, stable message (no need to string-match on err.Error()).
//
// The output is always a fully populated, canonical RoomConfig JSON —
// safe to store verbatim in rooms.config with no further massaging.
func (b Bounds) ValidateAndFill(raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		RoundDurationSeconds  *int  `json:"round_duration_seconds"`
		VotingDurationSeconds *int  `json:"voting_duration_seconds"`
		RoundCount            *int  `json:"round_count"`
		HostPaced             *bool `json:"host_paced"`
		JokerCount            *int  `json:"joker_count"`
		AllowSkipVote         *bool `json:"allow_skip_vote"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, &ValidationError{Field: "config", Reason: "invalid JSON: " + err.Error()}
		}
	}
	out := RoomConfig{
		RoundDurationSeconds:  intOr(in.RoundDurationSeconds, b.DefaultRoundDurationSeconds),
		VotingDurationSeconds: intOr(in.VotingDurationSeconds, b.DefaultVotingDurationSeconds),
		RoundCount:            intOr(in.RoundCount, b.DefaultRoundCount),
		HostPaced:             in.HostPaced != nil && *in.HostPaced,
		AllowSkipVote:         in.AllowSkipVote == nil || *in.AllowSkipVote,
	}
	// joker_count default depends on round_count, so it is computed after
	// round_count has been populated from input-or-default. ceil(n/5) via
	// integer arithmetic: (n + 4) / 5.
	if in.JokerCount != nil {
		out.JokerCount = *in.JokerCount
	} else {
		out.JokerCount = (out.RoundCount + 4) / 5
	}
	if out.RoundDurationSeconds < b.MinRoundDurationSeconds || out.RoundDurationSeconds > b.MaxRoundDurationSeconds {
		return nil, &ValidationError{Field: "round_duration_seconds", Reason: fmt.Sprintf("must be between %d and %d seconds", b.MinRoundDurationSeconds, b.MaxRoundDurationSeconds)}
	}
	if out.VotingDurationSeconds < b.MinVotingDurationSeconds || out.VotingDurationSeconds > b.MaxVotingDurationSeconds {
		return nil, &ValidationError{Field: "voting_duration_seconds", Reason: fmt.Sprintf("must be between %d and %d seconds", b.MinVotingDurationSeconds, b.MaxVotingDurationSeconds)}
	}
	if out.RoundCount < b.MinRoundCount || out.RoundCount > b.MaxRoundCount {
		return nil, &ValidationError{Field: "round_count", Reason: fmt.Sprintf("must be between %d and %d", b.MinRoundCount, b.MaxRoundCount)}
	}
	if out.JokerCount < 0 || out.JokerCount > out.RoundCount {
		return nil, &ValidationError{Field: "joker_count", Reason: fmt.Sprintf("must be between 0 and %d", out.RoundCount)}
	}
	return json.Marshal(out)
}

// MergeJSON overlays `patch` onto `base` at the top level and returns the
// combined JSON. Used by PATCH /rooms/{code}/config so clients can send a
// partial config (just the field they changed) without risking overwriting
// the other fields with zero values. Only the top-level keys are merged;
// nested objects are replaced wholesale (acceptable because our RoomConfig
// is flat by contract).
func MergeJSON(base, patch json.RawMessage) (json.RawMessage, error) {
	b := map[string]any{}
	p := map[string]any{}
	if len(base) > 0 {
		if err := json.Unmarshal(base, &b); err != nil {
			return nil, fmt.Errorf("base config: %w", err)
		}
	}
	if len(patch) > 0 {
		if err := json.Unmarshal(patch, &p); err != nil {
			return nil, fmt.Errorf("patch: %w", err)
		}
	}
	for k, v := range p {
		b[k] = v
	}
	return json.Marshal(b)
}

// ValidationError lets callers (API layer) distinguish bounds violations
// from malformed JSON and produce a stable error `code` per field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func intOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}
