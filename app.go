package main

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HarbourMasters/Sail/internal/config"
	"github.com/HarbourMasters/Sail/internal/sail"
	"github.com/HarbourMasters/Sail/internal/twitchapi"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound application: it owns the Sail TCP server, the Twitch
// session/EventSub connection, and the on-disk config.
type App struct {
	ctx context.Context

	configStore *config.Store
	configMu    sync.Mutex
	config      config.Config

	cooldownMu sync.Mutex
	cooldowns  map[string]time.Time

	// activityMu guards recentActivity, a newest-first buffer of fired
	// triggers. The live "activity" event only reaches current listeners; this
	// buffer repopulates the feed on reopen and catches what fired while closed.
	activityMu     sync.Mutex
	recentActivity []ActivityEvent

	sailServer    *sail.Server
	serverRunning bool

	// actionsMu guards lastActions, the game's action catalog cached from the
	// last connection. No static fallback — empty until a game connects.
	actionsMu   sync.Mutex
	lastActions []sail.ActionInfo

	// hooksMu guards lastHooks, the hook catalog the editor offers: the
	// compiled-in sail.KnownHookCatalog until a game's live hook.list replaces
	// it (refreshHookCatalog).
	hooksMu   sync.Mutex
	lastHooks []sail.HookInfo

	// hookSubMu guards the hook-subscription bookkeeping. Sail subscribes to a
	// hook only when something needs it (a binding, or the Hooks page
	// listening) — OnActorInit/OnFlagSet fire far too often to stream blindly.
	// watchedHooks is what the frontend watches live; appliedHooks is what
	// clients are subscribed to (bindings ∪ watched), tracked so a change sends
	// only the delta and never a double-subscribe.
	hookSubMu    sync.Mutex
	watchedHooks map[string]bool
	appliedHooks map[string]bool
	// feedActive mirrors len(watchedHooks) > 0 for a lock-free check on the hot
	// hook path.
	feedActive atomic.Bool

	twitchMu        sync.Mutex
	session         *twitchapi.StoredSession
	eventSub        *twitchapi.EventSubClient
	lastEventSubErr string

	// sentChatMu guards sentChat (posted text -> when). Twitch echoes our own
	// messages back over EventSub, so this lets handleChatMessage skip them —
	// else a binding posting a command-like message loops on itself.
	sentChatMu sync.Mutex
	sentChat   map[string]time.Time
}

// NewApp returns an App ready for startup.
func NewApp() *App {
	return &App{
		cooldowns:    make(map[string]time.Time),
		lastHooks:    sail.KnownHookCatalog(),
		watchedHooks: make(map[string]bool),
		appliedHooks: make(map[string]bool),
		sentChat:     make(map[string]time.Time),
	}
}

// startup loads persisted config and Twitch session, then brings up the game
// server and Twitch connection. ctx is saved for runtime calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	store, err := config.NewStore()
	if err != nil {
		log.Printf("could not open config store, using defaults: %v", err)
		a.config = config.Default()
	} else {
		a.configStore = store
		cfg, err := store.Load()
		if err != nil {
			log.Printf("could not load config, using defaults: %v", err)
			cfg = config.Default()
		}
		a.config = cfg
	}

	a.sailServer = sail.NewServer()
	a.wireSailServer()
	if err := a.StartServer(); err != nil {
		log.Printf("could not start sail server: %v", err)
	}

	// Off the startup path: validating over the network shouldn't block, and
	// the frontend handles Twitch status arriving late (twitch:status).
	go a.restoreTwitchSession()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopEventSub()
	if a.sailServer != nil {
		_ = a.sailServer.Stop()
	}
}

func (a *App) checkCooldown(key string, seconds float64) bool {
	if seconds <= 0 {
		return true
	}

	a.cooldownMu.Lock()
	defer a.cooldownMu.Unlock()

	if until, ok := a.cooldowns[key]; ok && time.Now().Before(until) {
		return false
	}
	a.cooldowns[key] = time.Now().Add(time.Duration(seconds * float64(time.Second)))
	return true
}

// maxRecentActivity bounds the buffer and Dashboard feed; the frontend caps
// its live feed to match.
const maxRecentActivity = 20

// ActivityEvent is a fired trigger, pushed live on the "activity" event and
// buffered in recentActivity for GetRecentActivity.
type ActivityEvent struct {
	Source  string `json:"source"` // "chat" | "redeem" | "hook"
	User    string `json:"user"`   // empty for a hook (no viewer behind it)
	Trigger string `json:"trigger"`
	// Error is set when the binding's script failed; it rides the feed so a
	// broken script shows on the Dashboard, not just a hidden log.
	Error string `json:"error,omitempty"`
	At    string `json:"at"`
}

func (a *App) emitActivity(event ActivityEvent) {
	event.At = time.Now().Format(time.RFC3339)

	// Record before emitting so the buffer has the event before any listener.
	a.activityMu.Lock()
	a.recentActivity = append([]ActivityEvent{event}, a.recentActivity...)
	if len(a.recentActivity) > maxRecentActivity {
		a.recentActivity = a.recentActivity[:maxRecentActivity]
	}
	a.activityMu.Unlock()

	runtime.EventsEmit(a.ctx, "activity", event)
}

// GetRecentActivity returns recent firings, newest first, so the Dashboard can
// repopulate its feed when reopened. Never nil — a nil slice serializes as
// JSON null.
func (a *App) GetRecentActivity() []ActivityEvent {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	return nonNilCopy(a.recentActivity)
}
