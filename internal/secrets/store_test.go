package secrets

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestLoadOBSPasswordReturnsEmptyWhenMissing(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	keyringGet = func(service, user string) (string, error) {
		if service != serviceName || user != obsPasswordUser {
			t.Fatalf("unexpected get args: %s %s", service, user)
		}
		return "", keyring.ErrNotFound
	}

	password, err := NewStore().LoadOBSPassword()
	if err != nil {
		t.Fatalf("LoadOBSPassword: %v", err)
	}
	if password != "" {
		t.Fatalf("password = %q", password)
	}
}

func TestSaveOBSPasswordDeletesWhenEmpty(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	deleted := false
	keyringDelete = func(service, user string) error {
		deleted = true
		if service != serviceName || user != obsPasswordUser {
			t.Fatalf("unexpected delete args: %s %s", service, user)
		}
		return nil
	}

	if err := NewStore().SaveOBSPassword(""); err != nil {
		t.Fatalf("SaveOBSPassword: %v", err)
	}
	if !deleted {
		t.Fatalf("expected delete on empty password")
	}
}

func TestLoadTwitchTokensUnmarshalsStoredJSON(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	keyringGet = func(service, user string) (string, error) {
		if service != serviceName || user != twitchTokensUser {
			t.Fatalf("unexpected get args: %s %s", service, user)
		}
		return `{"accessToken":"a","refreshToken":"r","tokenExpiry":"t"}`, nil
	}

	tokens, err := NewStore().LoadTwitchTokens()
	if err != nil {
		t.Fatalf("LoadTwitchTokens: %v", err)
	}
	if tokens.AccessToken != "a" || tokens.RefreshToken != "r" || tokens.TokenExpiry != "t" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
}

func TestSaveTwitchTokensWritesJSON(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	savedValue := ""
	keyringSet = func(service, user, value string) error {
		if service != serviceName || user != twitchTokensUser {
			t.Fatalf("unexpected set args: %s %s", service, user)
		}
		savedValue = value
		return nil
	}

	err := NewStore().SaveTwitchTokens(TwitchTokens{AccessToken: "a", RefreshToken: "r", TokenExpiry: "t"})
	if err != nil {
		t.Fatalf("SaveTwitchTokens: %v", err)
	}
	if savedValue != `{"accessToken":"a","refreshToken":"r","tokenExpiry":"t"}` {
		t.Fatalf("savedValue = %q", savedValue)
	}
}

func TestDeleteTwitchTokensIgnoresNotFound(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	keyringDelete = func(service, user string) error {
		if service != serviceName || user != twitchTokensUser {
			t.Fatalf("unexpected delete args: %s %s", service, user)
		}
		return keyring.ErrNotFound
	}

	if err := NewStore().DeleteTwitchTokens(); err != nil {
		t.Fatalf("DeleteTwitchTokens: %v", err)
	}
}

func TestLoadTwitchTokensReturnsInvalidJSONError(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	keyringGet = func(service, user string) (string, error) {
		return "{bad", nil
	}

	_, err := NewStore().LoadTwitchTokens()
	if err == nil {
		t.Fatalf("expected invalid json error")
	}
}

func TestLoadOBSPasswordReturnsUnderlyingError(t *testing.T) {
	restore := stubKeyring(t)
	defer restore()

	keyringGet = func(service, user string) (string, error) {
		return "", errors.New("keyring offline")
	}

	_, err := NewStore().LoadOBSPassword()
	if err == nil || err.Error() != "keyring offline" {
		t.Fatalf("expected underlying error, got %v", err)
	}
}

func stubKeyring(t *testing.T) func() {
	t.Helper()
	previousGet := keyringGet
	previousSet := keyringSet
	previousDelete := keyringDelete
	return func() {
		keyringGet = previousGet
		keyringSet = previousSet
		keyringDelete = previousDelete
	}
}
