package main

import (
	"fmt"
	"strings"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/sail"
	"github.com/google/uuid"
)

// persistLocked writes the config to disk. Callers must hold configMu.
func (a *App) persistLocked() error {
	if a.configStore == nil {
		return nil
	}
	if err := a.configStore.Save(a.config); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// ListCommands returns the configured chat commands.
func (a *App) ListCommands() []config.Command {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	return nonNilCopy(a.config.Commands)
}

// SaveCommand creates or replaces (by Trigger, case-insensitively) a chat
// command binding.
func (a *App) SaveCommand(cmd config.Command) error {
	trigger := strings.TrimSpace(cmd.Trigger)
	if trigger == "" {
		return fmt.Errorf("command trigger cannot be empty")
	}
	cmd.Trigger = trigger

	if err := validateScript(cmd.Binding.Script); err != nil {
		return fmt.Errorf("script has a syntax error: %w", err)
	}

	a.configMu.Lock()
	defer a.configMu.Unlock()

	a.config.Commands = upsert(a.config.Commands, func(c config.Command) bool {
		return strings.EqualFold(c.Trigger, trigger)
	}, cmd)
	return a.persistLocked()
}

// DeleteCommand removes a chat command binding by trigger.
func (a *App) DeleteCommand(trigger string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	a.config.Commands = removeIf(a.config.Commands, func(c config.Command) bool {
		return strings.EqualFold(c.Trigger, trigger)
	})
	return a.persistLocked()
}

// ListRedeems returns the configured channel point redeem bindings.
func (a *App) ListRedeems() []config.Redeem {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	return nonNilCopy(a.config.Redeems)
}

// SaveRedeem creates or replaces (by RewardID) a channel point redeem
// binding.
func (a *App) SaveRedeem(redeem config.Redeem) error {
	rewardID := strings.TrimSpace(redeem.RewardID)
	if rewardID == "" {
		return fmt.Errorf("redeem must have a reward id")
	}
	redeem.RewardID = rewardID

	if err := validateScript(redeem.Binding.Script); err != nil {
		return fmt.Errorf("script has a syntax error: %w", err)
	}

	a.configMu.Lock()
	defer a.configMu.Unlock()

	a.config.Redeems = upsert(a.config.Redeems, func(r config.Redeem) bool {
		return r.RewardID == rewardID
	}, redeem)
	return a.persistLocked()
}

// DeleteRedeem removes a channel point redeem binding by reward ID.
func (a *App) DeleteRedeem(rewardID string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	a.config.Redeems = removeIf(a.config.Redeems, func(r config.Redeem) bool {
		return r.RewardID == rewardID
	})
	return a.persistLocked()
}

// ListHookBindings returns the configured game-hook bindings.
func (a *App) ListHookBindings() []config.HookBinding {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	return nonNilCopy(a.config.Hooks)
}

// SaveHookBinding creates or replaces a game-hook binding: no ID means new (one
// is generated), an existing ID replaces. Identity is a generated ID, not the
// hook name, since one hook can carry several bindings.
func (a *App) SaveHookBinding(hb config.HookBinding) error {
	if strings.TrimSpace(hb.HookName) == "" {
		return fmt.Errorf("hook binding must have a hook")
	}
	if err := validateScript(hb.Binding.Script); err != nil {
		return fmt.Errorf("script has a syntax error: %w", err)
	}
	// An id filter on a non-filterable hook never matches (silent never-fire),
	// so drop it. Guards a hand-edited config; the editor prevents this too.
	if _, filterable := (sail.Hook{Type: hb.HookName}).FilterID(); !filterable {
		hb.IDFilter = nil
	}
	if hb.ID == "" {
		hb.ID = uuid.NewString()
	}

	a.configMu.Lock()
	a.config.Hooks = upsert(a.config.Hooks, func(h config.HookBinding) bool {
		return h.ID == hb.ID
	}, hb)
	err := a.persistLocked()
	a.configMu.Unlock()
	if err != nil {
		return err
	}

	// Reconcile outside the config lock: a newly-bound hook needs subscribing.
	a.reconcileHookSubscriptions()
	return nil
}

// DeleteHookBinding removes a binding by ID, then unsubscribes its hook if
// nothing else needs it.
func (a *App) DeleteHookBinding(id string) error {
	a.configMu.Lock()
	a.config.Hooks = removeIf(a.config.Hooks, func(h config.HookBinding) bool {
		return h.ID == id
	})
	err := a.persistLocked()
	a.configMu.Unlock()
	if err != nil {
		return err
	}

	a.reconcileHookSubscriptions()
	return nil
}

// ListHooks returns the hook catalog a binding can fire on. Available before a
// game connects (compiled-in sail.KnownHookCatalog); a live hook.list replaces
// it on connect (refreshHookCatalog).
func (a *App) ListHooks() []sail.HookInfo {
	a.hooksMu.Lock()
	defer a.hooksMu.Unlock()
	return nonNilCopy(a.lastHooks)
}

// ListActions returns the cached action catalog (refreshActionCatalog). Empty
// until a game connects — action.list needs an active connection.
func (a *App) ListActions() []sail.ActionInfo {
	a.actionsMu.Lock()
	defer a.actionsMu.Unlock()
	return nonNilCopy(a.lastActions)
}
