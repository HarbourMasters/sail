package main

import (
	"context"
	"fmt"
	"log"

	"github.com/HarbourMasters/Sail/internal/sail"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ServerStatus is the game TCP server's state, returned by GetServerStatus and
// pushed on the "server:status" event.
type ServerStatus struct {
	Running          bool `json:"running"`
	Port             int  `json:"port"`
	ConnectedClients int  `json:"connectedClients"`
}

func (a *App) wireSailServer() {
	a.sailServer.OnConnect = func(c *sail.Client) {
		log.Printf("game connected from %s", c.RemoteAddr())
		a.emitServerStatus()
		go a.refreshHookCatalog(c)
		go a.subscribeNewClient(c)
		go a.refreshActionCatalog(c)
	}
	a.sailServer.OnDisconnect = func(c *sail.Client) {
		log.Printf("game disconnected (client %d)", c.ID)
		a.emitServerStatus()
	}
	a.sailServer.OnHook = func(c *sail.Client, hook sail.Hook) {
		// Forward a raw hook to the frontend only while the page is listening
		// for it. feedActive keeps the not-listening path lock-free; the watched
		// check keeps a bound high-frequency hook from flooding the webview
		// during an unrelated listen. Bindings fire regardless.
		if a.feedActive.Load() && a.isHookWatched(hook.Type) {
			runtime.EventsEmit(a.ctx, "sail:hook", hook)
		}
		a.handleHook(hook)
	}
	a.sailServer.OnActionEnded = func(c *sail.Client, id string, outcome sail.Outcome) {
		log.Printf("client %d action %s ended: %s", c.ID, id, outcome)
	}
	a.sailServer.OnAcceptError = func(err error) {
		log.Printf("game server stopped unexpectedly: %v", err)
		a.serverRunning = false
		a.emitServerStatus()
	}
}

func (a *App) emitServerStatus() {
	runtime.EventsEmit(a.ctx, "server:status", a.GetServerStatus())
}

// refreshHookCatalog fetches a connected game's live hook catalog and pushes it
// to the frontend, so the editor offers exactly what the game supports (falling
// back to the compiled-in catalog). Subscribing is separate (subscribeNewClient)
// — Sail never subscribes to the whole catalog.
func (a *App) refreshHookCatalog(c *sail.Client) {
	hooks, err := c.ListHooks(context.Background())
	if err != nil || len(hooks) == 0 {
		if err != nil {
			log.Printf("fetch hook catalog from client %d failed: %v", c.ID, err)
		}
		hooks = sail.KnownHookCatalog()
	}

	a.hooksMu.Lock()
	a.lastHooks = hooks
	a.hooksMu.Unlock()
	runtime.EventsEmit(a.ctx, "hooks:updated", hooks)
}

// refreshActionCatalog fetches the game's action catalog, caches it for
// ListActions, and pushes it on its own "actions:updated" event — "server:status"
// fires synchronously in OnConnect before this runs, so a frontend refetching
// only on that would race ahead of the cache.
func (a *App) refreshActionCatalog(c *sail.Client) {
	actions, err := c.ListActions(context.Background())
	if err != nil {
		log.Printf("fetch action catalog from client %d failed: %v", c.ID, err)
		return
	}

	a.actionsMu.Lock()
	a.lastActions = actions
	a.actionsMu.Unlock()
	runtime.EventsEmit(a.ctx, "actions:updated", actions)
}

// configuredPort returns the game server's configured port.
func (a *App) configuredPort() int {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	return a.config.Port
}

// GetServerStatus reports whether the game server is listening and how many
// game clients are currently connected.
func (a *App) GetServerStatus() ServerStatus {
	return ServerStatus{
		Running:          a.serverRunning,
		Port:             a.configuredPort(),
		ConnectedClients: a.sailServer.ClientCount(),
	}
}

// StartServer starts listening for game connections on the configured port.
func (a *App) StartServer() error {
	if a.serverRunning {
		return nil
	}

	if err := a.sailServer.Start(a.configuredPort()); err != nil {
		a.serverRunning = false
		a.emitServerStatus()
		return fmt.Errorf("start game server: %w", err)
	}

	a.serverRunning = true
	a.emitServerStatus()
	return nil
}

// StopServer stops listening and disconnects clients. The same *sail.Server is
// reused for the next StartServer.
func (a *App) StopServer() error {
	if !a.serverRunning {
		return nil
	}

	err := a.sailServer.Stop()
	a.serverRunning = false
	a.emitServerStatus()
	if err != nil {
		return fmt.Errorf("stop game server: %w", err)
	}
	return nil
}

// SetPort changes the game server's port, persists it, and restarts the
// server on the new port if it was running.
func (a *App) SetPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	wasRunning := a.serverRunning
	if wasRunning {
		if err := a.StopServer(); err != nil {
			return err
		}
	}

	if err := func() error {
		a.configMu.Lock()
		defer a.configMu.Unlock()
		a.config.Port = port
		return a.persistLocked()
	}(); err != nil {
		return err
	}

	if wasRunning {
		return a.StartServer()
	}
	return nil
}
