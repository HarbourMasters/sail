package twitchapi

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "dev.harbourmasters.sail"
	keyringUser    = "twitch-session"
)

// StoredSession is the logged-in Twitch identity kept in the OS keychain
// between runs, rather than in the plaintext config file.
type StoredSession struct {
	AccessToken string   `json:"accessToken"`
	Login       string   `json:"login"`
	UserID      string   `json:"userId"`
	Scopes      []string `json:"scopes"`
}

// SaveSession stores session in the OS keychain, replacing any previous one.
func SaveSession(session StoredSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode twitch session: %w", err)
	}
	if err := keyring.Set(keyringService, keyringUser, string(data)); err != nil {
		return fmt.Errorf("save twitch session to keychain: %w", err)
	}
	return nil
}

// LoadSession returns the stored session, or nil if the user hasn't logged
// in (or has logged out) on this machine.
func LoadSession() (*StoredSession, error) {
	data, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read twitch session from keychain: %w", err)
	}

	var session StoredSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("decode twitch session: %w", err)
	}
	return &session, nil
}

// ClearSession removes any stored session, e.g. on logout.
func ClearSession() error {
	err := keyring.Delete(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
