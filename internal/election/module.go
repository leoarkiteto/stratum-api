package election

import (
	"github.com/leoarkiteto/stratum/internal/session"
	"github.com/leoarkiteto/stratum/internal/web"
)

// Deps are the election module's dependencies.
type Deps struct {
	Store    ElectionStore
	Sessions session.Store
	Users    web.UserStore
	Secure   bool
}

// New builds the election module handler.
func New(deps Deps) *Handler {
	return &Handler{
		service:  NewService(deps.Store),
		sessions: deps.Sessions,
		users:    deps.Users,
		secure:   deps.Secure,
	}
}
