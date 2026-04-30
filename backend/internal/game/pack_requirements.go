// backend/internal/game/pack_requirements.go
package game

// PackRole names a content stream a game type needs (image, text, audio…).
// Roles are strings so they can round-trip through YAML and JSON without a
// lookup table; keep the set small and add new ones deliberately.
type PackRole string

const (
	PackRoleImage  PackRole = "image"
	PackRoleText   PackRole = "text"
	PackRolePrompt PackRole = "prompt"
	PackRoleFiller PackRole = "filler"
)

// PackRequirement describes one pack a game type needs to run a room.
// MinItemsFn is Go-only (not YAML) because the arithmetic depends on runtime
// config (round_count, hand_size) and max_players — values the manifest does
// not express directly.
type PackRequirement struct {
	Role            PackRole
	PayloadVersions []int
	MinItemsFn      func(cfg RoomConfig, maxPlayers int) int
}

// ManifestPackRequirement is the YAML/JSON projection of PackRequirement:
// roles and payload versions only. The handler's RequiredPacks() method reads
// this list from the manifest and attaches MinItemsFn at runtime.
type ManifestPackRequirement struct {
	Role            PackRole `yaml:"role"             json:"role"`
	PayloadVersions []int    `yaml:"payload_versions" json:"payload_versions"`
}
