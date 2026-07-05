package ticktick

import (
	"fmt"
	"os"

	"github.com/j4y-w4lk3r/ttcli/internal/secrets"
)

// CredentialOptions selects where login email/password come from.
// Default: discover a 1Password Login item titled "TickTick" in any vault.
type CredentialOptions struct {
	Vault    string
	Item     string
	Email    string // explicit override (flags only; not read from env)
	Password string
	Backend  secrets.Backend // nil → secrets.Default()
}

func (o CredentialOptions) backend() secrets.Backend {
	if o.Backend != nil {
		return o.Backend
	}
	return secrets.Default()
}

func (o CredentialOptions) vault() string {
	if o.Vault != "" {
		return o.Vault
	}
	return os.Getenv("TTCLI_OP_VAULT")
}

func (o CredentialOptions) item() string {
	if o.Item != "" {
		return o.Item
	}
	if v := os.Getenv("TTCLI_OP_ITEM"); v != "" {
		return v
	}
	return "TickTick"
}

// Resolve returns TickTick credentials. Explicit email/password win;
// otherwise credentials are read from 1Password.
func (o CredentialOptions) Resolve() (Credentials, error) {
	if o.Email != "" || o.Password != "" {
		c := Credentials{Email: o.Email, Password: o.Password}
		if !c.Valid() {
			return Credentials{}, fmt.Errorf("incomplete credentials: pass both --email and --password, or omit both to use 1Password")
		}
		return c, nil
	}

	b := o.backend()
	if err := b.Available(); err != nil {
		return Credentials{}, err
	}
	if err := b.CheckSignedIn(); err != nil {
		return Credentials{}, err
	}
	vault, item := o.vault(), o.item()
	user, pass, err := b.GetLogin(vault, item)
	if err != nil {
		hint := fmt.Sprintf("ensure a Login item titled %q exists in 1Password", item)
		if vault != "" {
			hint = fmt.Sprintf("ensure a Login item titled %q exists in vault %q", item, vault)
		}
		return Credentials{}, fmt.Errorf("1Password login lookup failed: %w\n  %s (or pass --vault/--item)", err, hint)
	}
	return Credentials{Email: user, Password: pass}, nil
}
