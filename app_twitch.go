package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/twitchapi"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// chatWriteScope is required to post chat. Added after the read-only scopes,
	// so a pre-existing session needs a fresh login (surfaced as CanPostChat).
	chatWriteScope = "user:write:chat"

	// ownEchoWindow is how long after posting Sail treats a matching message
	// from itself as the echo rather than a new trigger — longer than the send
	// + EventSub round-trip.
	ownEchoWindow = 15 * time.Second
)

// TwitchStatus is the logged-in Twitch identity (if any), returned by
// GetTwitchStatus and pushed on the "twitch:status" event.
type TwitchStatus struct {
	LoggedIn      bool   `json:"loggedIn"`
	Login         string `json:"login"`
	NeedsClientID bool   `json:"needsClientId"`
	// CanPostChat is whether the token carries the chat-write scope, so the UI
	// can prompt a re-login for a session that predates it.
	CanPostChat   bool   `json:"canPostChat"`
	EventSubError string `json:"eventSubError,omitempty"`
}

// restoreTwitchSession re-validates a stored session at startup. Implicit-grant
// tokens have no refresh token, so an expired one just needs a fresh login.
func (a *App) restoreTwitchSession() {
	session, err := twitchapi.LoadSession()
	if err != nil {
		log.Printf("could not read stored twitch session: %v", err)
		return
	}
	if session == nil {
		return
	}

	validation, err := twitchapi.ValidateToken(context.Background(), session.AccessToken)
	if err != nil {
		log.Printf("stored twitch session is no longer valid, please log in again: %v", err)
		_ = twitchapi.ClearSession()
		return
	}
	session.Login = validation.Login
	session.UserID = validation.UserID
	session.Scopes = validation.Scopes

	a.twitchMu.Lock()
	a.session = session
	a.twitchMu.Unlock()

	a.startEventSub()
	a.emitTwitchStatus()
}

func (a *App) emitTwitchStatus() {
	runtime.EventsEmit(a.ctx, "twitch:status", a.GetTwitchStatus())
}

// currentSession returns the logged-in session, or nil.
func (a *App) currentSession() *twitchapi.StoredSession {
	a.twitchMu.Lock()
	defer a.twitchMu.Unlock()
	return a.session
}

// GetTwitchStatus reports whether a Twitch account is logged in.
func (a *App) GetTwitchStatus() TwitchStatus {
	a.twitchMu.Lock()
	defer a.twitchMu.Unlock()

	status := TwitchStatus{NeedsClientID: !twitchapi.Configured(), EventSubError: a.lastEventSubErr}
	if a.session != nil {
		status.LoggedIn = true
		status.Login = a.session.Login
		status.CanPostChat = slices.Contains(a.session.Scopes, chatWriteScope)
	}
	return status
}

// LoginWithTwitch opens the system browser to Twitch's login page and
// blocks until the user finishes (or cancels, or five minutes pass).
func (a *App) LoginWithTwitch() error {
	if !twitchapi.Configured() {
		return fmt.Errorf("this build has no Twitch client ID configured yet — see the README to register one")
	}

	authenticator := &twitchapi.Authenticator{
		ClientID:    twitchapi.ClientID(),
		RedirectURI: twitchapi.RedirectURI,
		Scopes:      twitchapi.Scopes,
	}

	result, err := authenticator.Login(a.ctx, func(loginURL string) error {
		runtime.BrowserOpenURL(a.ctx, loginURL)
		return nil
	})
	if err != nil {
		return fmt.Errorf("twitch login: %w", err)
	}

	session := twitchapi.StoredSession{
		AccessToken: result.AccessToken,
		Login:       result.Login,
		UserID:      result.UserID,
		Scopes:      result.Scopes,
	}
	if err := twitchapi.SaveSession(session); err != nil {
		log.Printf("could not persist twitch session to keychain, you'll need to log in again next launch: %v", err)
	}

	a.stopEventSub()
	a.twitchMu.Lock()
	a.session = &session
	a.twitchMu.Unlock()

	a.startEventSub()
	a.emitTwitchStatus()
	return nil
}

// LogoutFromTwitch disconnects from EventSub and forgets the stored
// session.
func (a *App) LogoutFromTwitch() error {
	a.stopEventSub()

	a.twitchMu.Lock()
	a.session = nil
	a.twitchMu.Unlock()

	if err := twitchapi.ClearSession(); err != nil {
		return fmt.Errorf("clear twitch session: %w", err)
	}

	a.emitTwitchStatus()
	return nil
}

// ListTwitchRewards fetches the broadcaster's current channel point rewards
// from Twitch, for the Redeems page to map onto effects.
func (a *App) ListTwitchRewards() ([]twitchapi.CustomReward, error) {
	session := a.currentSession()
	if session == nil {
		return nil, fmt.Errorf("log in to twitch first")
	}

	helix := &twitchapi.Helix{ClientID: twitchapi.ClientID(), AccessToken: session.AccessToken}
	rewards, err := helix.GetCustomRewards(context.Background(), session.UserID)
	if err != nil {
		return nil, fmt.Errorf("list twitch rewards: %w", err)
	}
	return rewards, nil
}

func (a *App) startEventSub() {
	session := a.currentSession()
	if session == nil {
		return
	}

	client := &twitchapi.EventSubClient{
		ClientID:      twitchapi.ClientID(),
		AccessToken:   session.AccessToken,
		BroadcasterID: session.UserID,
		OnChatMessage: a.handleChatMessage,
		OnRedemption:  a.handleRedemption,
		OnConnected: func() {
			a.setEventSubError("")
		},
		OnError: func(err error) {
			log.Printf("twitch eventsub error: %v", err)
			a.setEventSubError(err.Error())
		},
	}

	a.twitchMu.Lock()
	a.eventSub = client
	a.twitchMu.Unlock()

	client.Start(a.ctx)
}

func (a *App) stopEventSub() {
	a.twitchMu.Lock()
	client := a.eventSub
	a.eventSub = nil
	a.twitchMu.Unlock()

	if client != nil {
		client.Stop()
	}
	a.setEventSubError("")
}

// setEventSubError records the latest EventSub error (or clears it, "") so the
// Dashboard can show it, not just the log.
func (a *App) setEventSubError(message string) {
	a.twitchMu.Lock()
	changed := a.lastEventSubErr != message
	a.lastEventSubErr = message
	a.twitchMu.Unlock()

	if changed {
		a.emitTwitchStatus()
	}
}

func (a *App) handleChatMessage(event twitchapi.ChatMessageEvent) {
	// Skip chat Sail posted itself (echoed back over EventSub) — else a binding
	// posting a command-like message loops on itself.
	if a.isOwnEcho(event) {
		return
	}

	text := strings.TrimSpace(event.Message.Text)
	if text == "" {
		return
	}

	fields := strings.Fields(text)
	trigger, args := fields[0], fields[1:]

	binding, found := a.findCommandBinding(trigger)
	if !found {
		return
	}
	if !a.checkCooldown("command:"+strings.ToLower(trigger), binding.CooldownSeconds) {
		return
	}

	a.fireTrigger(
		binding,
		triggerContext{
			User:    event.ChatterUserName,
			Message: strings.TrimSpace(strings.TrimPrefix(text, trigger)),
			Args:    args,
		},
		ActivityEvent{Source: "chat", User: event.ChatterUserName, Trigger: trigger},
	)
}

func (a *App) handleRedemption(event twitchapi.RewardRedemptionEvent) {
	binding, found := a.findRedeemBinding(event.Reward.ID)
	if !found {
		return
	}
	if !a.checkCooldown("redeem:"+event.Reward.ID, binding.CooldownSeconds) {
		return
	}

	input := strings.TrimSpace(event.UserInput)
	a.fireTrigger(
		binding,
		triggerContext{User: event.UserName, Message: input, Args: strings.Fields(input)},
		ActivityEvent{Source: "redeem", User: event.UserName, Trigger: event.Reward.Title},
	)
}

func (a *App) findCommandBinding(trigger string) (config.Binding, bool) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	cmd, ok := find(a.config.Commands, func(c config.Command) bool { return strings.EqualFold(c.Trigger, trigger) })
	return cmd.Binding, ok
}

func (a *App) findRedeemBinding(rewardID string) (config.Binding, bool) {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	redeem, ok := find(a.config.Redeems, func(r config.Redeem) bool { return r.RewardID == rewardID })
	return redeem.Binding, ok
}

// postChatMessage posts to the broadcaster's own chat. Runs on its own
// goroutine off the trigger path (dispatchSteps queues it), being a network
// call. A missing login or chat-write scope is logged and skipped.
func (a *App) postChatMessage(message string) {
	session := a.currentSession()
	if session == nil {
		log.Printf("skipping chat message %q: not logged in to twitch", message)
		return
	}
	if !slices.Contains(session.Scopes, chatWriteScope) {
		log.Printf("skipping chat message %q: log out and back in to grant the %s permission", message, chatWriteScope)
		return
	}

	// Record before sending: the EventSub echo can arrive before this HTTP
	// response.
	a.rememberSentChat(message)

	helix := &twitchapi.Helix{ClientID: twitchapi.ClientID(), AccessToken: session.AccessToken}
	result, err := helix.SendChatMessage(context.Background(), session.UserID, session.UserID, message)
	if err != nil {
		log.Printf("post chat message failed: %v", err)
		return
	}
	if !result.Sent {
		log.Printf("twitch dropped chat message %q: %s", message, result.DropReason)
	}
}

// isOwnEcho reports whether an incoming message is Sail's own echo: same author
// (us) and a text we sent within ownEchoWindow. A viewer typing the same text
// (different author) still fires.
func (a *App) isOwnEcho(event twitchapi.ChatMessageEvent) bool {
	session := a.currentSession()
	if session == nil || event.ChatterUserID != session.UserID {
		return false
	}

	text := strings.TrimSpace(event.Message.Text)
	a.sentChatMu.Lock()
	defer a.sentChatMu.Unlock()
	sentAt, ok := a.sentChat[text]
	return ok && time.Since(sentAt) < ownEchoWindow
}

// rememberSentChat records a posted message, pruning stale entries so the map
// stays small.
func (a *App) rememberSentChat(text string) {
	trimmed := strings.TrimSpace(text)
	now := time.Now()

	a.sentChatMu.Lock()
	defer a.sentChatMu.Unlock()
	for t, at := range a.sentChat {
		if now.Sub(at) > ownEchoWindow {
			delete(a.sentChat, t)
		}
	}
	a.sentChat[trimmed] = now
}
