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
	value, err := keyring.Get(serviceName, obsPasswordUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func (s *Store) SaveOBSPassword(password string) error {
	if password == "" {
		return s.DeleteOBSPassword()
	}
	return keyring.Set(serviceName, obsPasswordUser, password)
}

func (s *Store) DeleteOBSPassword() error {
	err := keyring.Delete(serviceName, obsPasswordUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

func (s *Store) LoadTwitchTokens() (TwitchTokens, error) {
	value, err := keyring.Get(serviceName, twitchTokensUser)
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
	return keyring.Set(serviceName, twitchTokensUser, string(data))
}

func (s *Store) DeleteTwitchTokens() error {
	err := keyring.Delete(serviceName, twitchTokensUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
