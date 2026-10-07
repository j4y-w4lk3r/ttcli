// Package secrets reads TickTick login credentials from 1Password via the
// `op` CLI. Tests use the in-memory backend instead of touching a real vault.
package secrets

// Field is one labeled value on a 1Password item.
type Field struct {
	Label   string
	Purpose string
	Value   string
}

// Item is a 1Password item reduced to the fields a caller asked to read.
type Item struct {
	Title    string
	Category string
	Vault    string
	Fields   []Field
}

// Backend fetches a Login item's username and password.
type Backend interface {
	Available() error
	CheckSignedIn() error
	GetLogin(vault, itemRef string) (username, password string, err error)
	// ItemsNamed returns every item with that title, in any category.
	// vault limits the search when set.
	ItemsNamed(vault, title string) ([]Item, error)
}

// Default returns the production backend (1Password CLI).
func Default() Backend { return OnePassword{} }
