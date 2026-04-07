// backend/internal/game/registry.go
package game

import "fmt"

// Registry maps game type slugs to their handlers.
// Use NewRegistry() in main.go; call Register() for each game type.
type Registry struct {
	handlers map[string]GameTypeHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]GameTypeHandler)}
}

// Register adds a handler. Panics on duplicate slug (caught at startup, never at runtime).
func (r *Registry) Register(h GameTypeHandler) {
	if _, exists := r.handlers[h.Slug()]; exists {
		panic(fmt.Sprintf("game: duplicate handler for slug %q", h.Slug()))
	}
	r.handlers[h.Slug()] = h
}

// Get returns the handler for slug, or (nil, false) if not registered.
func (r *Registry) Get(slug string) (GameTypeHandler, bool) {
	h, ok := r.handlers[slug]
	return h, ok
}
