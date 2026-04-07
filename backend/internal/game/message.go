// backend/internal/game/message.go
package game

import "encoding/json"

// Message is the envelope for all WebSocket messages.
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}
