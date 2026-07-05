// Package secrets reads TickTick login credentials from 1Password via the
// `op` CLI. Tests use the in-memory backend instead of touching a real vault.
package secrets

// Backend fetches a Login item's username and password.
type Backend interface {
	Available() error
	CheckSignedIn() error
	GetLogin(vault, itemRef string) (username, password string, err error)
}

// Default returns the production backend (1Password CLI).
func Default() Backend { return OnePassword{} }
