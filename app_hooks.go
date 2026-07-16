package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/HarbourMasters/Sail/internal/sail"
)

// Hook-subscription management. Some hooks (OnActorInit, OnFlagSet) fire
// constantly, so Sail subscribes to a hook only when something needs it — a
// binding, or the Hooks page listening to discover ids. The desired set is
// (bindings ∪ watched); appliedHooks tracks what clients have, so each change
// is a minimal delta and never a double-subscribe (which double-fires).

// WatchHook subscribes connected games to a hook for the live feed, to discover
// the ids an id filter matches on. The opt-in counterpart to subscribing to
// bound hooks by default; the page unwatches when it stops or navigates away.
func (a *App) WatchHook(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("a hook name is required to listen")
	}
	a.hookSubMu.Lock()
	a.watchedHooks[name] = true
	a.feedActive.Store(len(a.watchedHooks) > 0)
	a.hookSubMu.Unlock()

	a.reconcileHookSubscriptions()
	return nil
}

// UnwatchHook stops observing a hook for the live feed. If a binding still
// needs it, it stays subscribed; otherwise the game is told to stop sending it.
func (a *App) UnwatchHook(name string) error {
	a.hookSubMu.Lock()
	delete(a.watchedHooks, name)
	a.feedActive.Store(len(a.watchedHooks) > 0)
	a.hookSubMu.Unlock()

	a.reconcileHookSubscriptions()
	return nil
}

// isHookWatched reports whether the feed is listening for a hook. Reached only
// after the lock-free feedActive check, so the lock is off the hot path.
func (a *App) isHookWatched(name string) bool {
	a.hookSubMu.Lock()
	defer a.hookSubMu.Unlock()
	return a.watchedHooks[name]
}

// boundHookNames is the set of hook names that have at least one binding.
func (a *App) boundHookNames() map[string]bool {
	a.configMu.Lock()
	defer a.configMu.Unlock()

	names := make(map[string]bool)
	for _, hb := range a.config.Hooks {
		names[hb.HookName] = true
	}
	return names
}

// subscribeNewClient brings a freshly connected game up to the desired set. It
// starts subscribed to nothing (games forget on disconnect), so it gets the
// whole set — never a duplicate.
func (a *App) subscribeNewClient(c *sail.Client) {
	bound := a.boundHookNames()

	a.hookSubMu.Lock()
	desired := union(bound, a.watchedHooks)
	a.appliedHooks = desired
	a.hookSubMu.Unlock()

	for name := range desired {
		go a.subscribeHook(c, name)
	}
}

// reconcileHookSubscriptions brings every connected game in line with the
// desired set (bindings ∪ watched), sending only the delta since the last
// reconcile. Call it whenever bindings or the watch set change.
func (a *App) reconcileHookSubscriptions() {
	bound := a.boundHookNames()
	clients := a.sailServer.Clients()

	a.hookSubMu.Lock()
	desired := union(bound, a.watchedHooks)
	add := difference(desired, a.appliedHooks)
	remove := difference(a.appliedHooks, desired)
	a.appliedHooks = desired
	a.hookSubMu.Unlock()

	for _, c := range clients {
		for name := range add {
			go a.subscribeHook(c, name)
		}
		for name := range remove {
			go a.unsubscribeHook(c, name)
		}
	}
}

func (a *App) subscribeHook(c *sail.Client, name string) {
	outcome, err := c.Subscribe(context.Background(), sail.HookName(name))
	if err != nil {
		log.Printf("subscribe %s on client %d failed: %v", name, c.ID, err)
		return
	}
	if outcome != sail.OutcomeOK {
		log.Printf("client %d responded %q to subscribe %s", c.ID, outcome, name)
	}
}

func (a *App) unsubscribeHook(c *sail.Client, name string) {
	outcome, err := c.Unsubscribe(context.Background(), sail.HookName(name))
	if err != nil {
		log.Printf("unsubscribe %s on client %d failed: %v", name, c.ID, err)
		return
	}
	if outcome != sail.OutcomeOK {
		log.Printf("client %d responded %q to unsubscribe %s", c.ID, outcome, name)
	}
}

// union returns a new set of every key in a or b.
func union(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// difference returns the keys in a that aren't in b.
func difference(a, b map[string]bool) map[string]bool {
	out := make(map[string]bool)
	for k := range a {
		if !b[k] {
			out[k] = true
		}
	}
	return out
}
