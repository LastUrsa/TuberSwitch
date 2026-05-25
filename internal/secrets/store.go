package secrets

import (
	"encoding/json"
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	serviceName      = "TuberSwitch"
	obsPasswordUser  = "obs-password"
	twitchTokensUser = "twitch-tokens"
)

var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

type Store struct{}

type TwitchTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenExpiry  string `json:"tokenExpiry"`
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) LoadOBSPassword() (string, error) {
	value, err := keyringGet(serviceName, obsPasswordUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func (s *Store) SaveOBSPassword(password string) error {
	if password == "" {
		return s.DeleteOBSPassword()
	}
	return keyringSet(serviceName, obsPasswordUser, password)
}

func (s *Store) DeleteOBSPassword() error {
	err := keyringDelete(serviceName, obsPasswordUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Store) LoadTwitchTokens() (TwitchTokens, error) {
	value, err := keyringGet(serviceName, twitchTokensUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return TwitchTokens{}, nil
	}
	if err != nil {
		return TwitchTokens{}, err
	}
	var tokens TwitchTokens
	if err := json.Unmarshal([]byte(value), &tokens); err != nil {
		return TwitchTokens{}, err
	}
	return tokens, nil
}

func (s *Store) SaveTwitchTokens(tokens TwitchTokens) error {
	if tokens.AccessToken == "" && tokens.RefreshToken == "" && tokens.TokenExpiry == "" {
		return s.DeleteTwitchTokens()
	}
	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	return keyringSet(serviceName, twitchTokensUser, string(data))
}

func (s *Store) DeleteTwitchTokens() error {
	err := keyringDelete(serviceName, twitchTokensUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
