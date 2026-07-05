package ticktick

import (
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/secrets"
)

func TestCredentialOptionsResolve_explicit(t *testing.T) {
	creds, err := CredentialOptions{Email: "a@b.c", Password: "pw"}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if creds.Email != "a@b.c" || creds.Password != "pw" {
		t.Fatalf("got %+v", creds)
	}
}

func TestCredentialOptionsResolve_partialExplicit(t *testing.T) {
	_, err := CredentialOptions{Email: "a@b.c"}.Resolve()
	if err == nil {
		t.Fatal("expected error for partial explicit creds")
	}
}

func TestCredentialOptionsResolve_onePassword(t *testing.T) {
	mem := secrets.NewMemory()
	mem.SetLogin("Private", "TickTick", "u@x.y", "secret")
	creds, err := CredentialOptions{Backend: mem}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if creds.Email != "u@x.y" || creds.Password != "secret" {
		t.Fatalf("got %+v", creds)
	}
}

func TestCredentialOptionsResolve_customVaultItem(t *testing.T) {
	mem := secrets.NewMemory()
	mem.SetLogin("Work", "My TickTick", "w@x.y", "pw")
	creds, err := CredentialOptions{
		Vault: "Work", Item: "My TickTick", Backend: mem,
	}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if creds.Email != "w@x.y" {
		t.Fatalf("got %+v", creds)
	}
}
