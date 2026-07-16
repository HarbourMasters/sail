package sail

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// shortTimeout is how long we wait for packet types the game answers
	// immediately (command, action.remove/list/status, hook.list, subscribe,
	// unsubscribe).
	shortTimeout = 5 * time.Second

	framesPerSecond = 20

	// defaultExpiresAfterFrames mirrors the game's DEFAULT_EXPIRES_AFTER
	// (Sail.cpp): how long it waits for an action to become ready when a
	// request doesn't say.
	defaultExpiresAfterFrames uint32 = 20 * 30 // 30 seconds

	// resultMargin is slack over the game's own wait, to absorb network latency
	// rather than timing out just as its answer arrives.
	resultMargin = 5 * time.Second
)

// Client is a single game connection. The game answers every command/action/
// query with a result carrying the same ID, may follow a timed action.apply
// with a later action.ended (same ID), and may send unsolicited hook packets.
type Client struct {
	ID   int
	conn net.Conn

	server *Server

	mu      sync.Mutex
	pending map[string]chan resultPacket

	done     chan struct{}
	closeErr sync.Once
}

func newClient(id int, conn net.Conn, server *Server) *Client {
	c := &Client{
		ID:      id,
		conn:    conn,
		server:  server,
		pending: make(map[string]chan resultPacket),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// RemoteAddr returns the address the game connected from.
func (c *Client) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// SendCommand sends a raw console command and waits for the game's result.
func (c *Client) SendCommand(ctx context.Context, command string) (Outcome, error) {
	id := uuid.NewString()
	framed, err := frame(commandPacket{ID: id, Type: PacketTypeCommand, Command: command})
	if err != nil {
		return "", fmt.Errorf("encode command packet: %w", err)
	}
	result, err := c.send(ctx, id, framed, shortTimeout)
	return result.Outcome, err
}

// ApplyAction starts (or re-triggers) an action, returning the id it was sent
// with and the immediate result. The reply waits until the action runs — up to
// req.ExpiresAfter (default 30s) if the game isn't ready. A timed action
// reports OutcomeApplied here; OutcomeFinished/Cancelled arrive later,
// asynchronously, via Server.OnActionEnded on this same id.
func (c *Client) ApplyAction(ctx context.Context, req ActionRequest) (id string, outcome Outcome, err error) {
	id = uuid.NewString()
	framed, err := frame(actionApplyPacket{
		ID:           id,
		Type:         PacketTypeActionApply,
		Name:         req.Name,
		Params:       req.Params,
		Duration:     req.Duration,
		ExpiresAfter: req.ExpiresAfter,
		Lifetime:     req.Lifetime,
	})
	if err != nil {
		return id, "", fmt.Errorf("encode action.apply packet: %w", err)
	}
	result, err := c.send(ctx, id, framed, applyActionTimeout(req.ExpiresAfter))
	return id, result.Outcome, err
}

// applyActionTimeout mirrors the game's readiness wait: ExpiresAfter 0 means
// wait forever (bounded only by ctx); else that many frames (20/sec) plus
// resultMargin for latency.
func applyActionTimeout(expiresAfter *uint32) time.Duration {
	frames := defaultExpiresAfterFrames
	if expiresAfter != nil {
		frames = *expiresAfter
	}
	if frames == 0 {
		return 0
	}
	return time.Duration(frames)*time.Second/framesPerSecond + resultMargin
}

// RemoveAction cancels every running instance of a named action, reporting
// how many instances were cancelled.
func (c *Client) RemoveAction(ctx context.Context, name string) (cancelled int, outcome Outcome, err error) {
	id := uuid.NewString()
	framed, err := frame(actionRemovePacket{ID: id, Type: PacketTypeActionRemove, Name: name})
	if err != nil {
		return 0, "", fmt.Errorf("encode action.remove packet: %w", err)
	}
	result, err := c.send(ctx, id, framed, shortTimeout)
	return result.Cancelled, result.Outcome, err
}

// ListActions fetches the game's live action catalog.
func (c *Client) ListActions(ctx context.Context) ([]ActionInfo, error) {
	result, err := c.sendQuery(ctx, PacketTypeActionList)
	if err != nil {
		return nil, err
	}
	return result.Actions, nil
}

// ActionStatusInfo is the game's answer to ActionStatus.
type ActionStatusInfo struct {
	Ready   bool
	Pending int
	Active  int
}

// ActionStatus reports whether the game can process actions now, and how many
// are pending/active — for holding a request back rather than eating an
// OutcomeExpired.
func (c *Client) ActionStatus(ctx context.Context) (ActionStatusInfo, error) {
	result, err := c.sendQuery(ctx, PacketTypeActionStatus)
	if err != nil {
		return ActionStatusInfo{}, err
	}
	return ActionStatusInfo{Ready: result.Ready, Pending: result.Pending, Active: result.Active}, nil
}

// ListHooks fetches the game's live hook catalog, enriching each hook with the
// field its id filter matches on (filterFieldFor). hook.list itself reports
// only name + idFilter, so this keeps FilterField sourced from the one Go
// mapping whether the catalog is live or compiled-in (KnownHookCatalog).
func (c *Client) ListHooks(ctx context.Context) ([]HookInfo, error) {
	result, err := c.sendQuery(ctx, PacketTypeHookList)
	if err != nil {
		return nil, err
	}
	hooks := result.Hooks
	for i := range hooks {
		hooks[i].FilterField = filterFieldFor(HookName(hooks[i].Name))
	}
	return hooks, nil
}

// sendQuery sends a no-argument query (action.list/status, hook.list) and
// returns its result, erroring (with the game's Reason) on a non-OK outcome —
// so a query method can't forget to check Outcome before trusting the reply.
func (c *Client) sendQuery(ctx context.Context, packetType PacketType) (resultPacket, error) {
	id := uuid.NewString()
	framed, err := frame(queryPacket{ID: id, Type: packetType})
	if err != nil {
		return resultPacket{}, fmt.Errorf("encode %s packet: %w", packetType, err)
	}
	result, err := c.send(ctx, id, framed, shortTimeout)
	if err != nil {
		return resultPacket{}, err
	}
	if result.Outcome != OutcomeOK {
		if result.Reason != "" {
			return resultPacket{}, fmt.Errorf("%s returned %q: %s", packetType, result.Outcome, result.Reason)
		}
		return resultPacket{}, fmt.Errorf("%s returned %q", packetType, result.Outcome)
	}
	return result, nil
}

// Subscribe registers interest in every occurrence of a hook. The game forgets
// subscriptions on disconnect, so call this again on each new connection.
func (c *Client) Subscribe(ctx context.Context, hook HookName) (Outcome, error) {
	return c.sendSubscription(ctx, PacketTypeSubscribe, hook, nil)
}

// SubscribeFiltered registers interest in a hook event narrowed to a single
// scene/item/actor id — only meaningful for hooks whose HookInfo.IDFilter
// is true (see ListHooks).
func (c *Client) SubscribeFiltered(ctx context.Context, hook HookName, idFilter int) (Outcome, error) {
	return c.sendSubscription(ctx, PacketTypeSubscribe, hook, &idFilter)
}

// Unsubscribe removes a previously registered unfiltered subscription.
func (c *Client) Unsubscribe(ctx context.Context, hook HookName) (Outcome, error) {
	return c.sendSubscription(ctx, PacketTypeUnsubscribe, hook, nil)
}

// UnsubscribeFiltered removes a previously registered filtered subscription.
// idFilter must match the value originally passed to SubscribeFiltered.
func (c *Client) UnsubscribeFiltered(ctx context.Context, hook HookName, idFilter int) (Outcome, error) {
	return c.sendSubscription(ctx, PacketTypeUnsubscribe, hook, &idFilter)
}

func (c *Client) sendSubscription(ctx context.Context, packetType PacketType, hook HookName, idFilter *int) (Outcome, error) {
	id := uuid.NewString()
	framed, err := frame(subscriptionPacket{ID: id, Type: packetType, HookName: hook, HookIDFilter: idFilter})
	if err != nil {
		return "", fmt.Errorf("encode %s packet: %w", packetType, err)
	}
	result, err := c.send(ctx, id, framed, shortTimeout)
	return result.Outcome, err
}

func frame(packet any) ([]byte, error) {
	payload, err := json.Marshal(packet)
	if err != nil {
		return nil, err
	}
	return append(payload, 0), nil
}

// send writes a framed packet and waits for its result. timeout <= 0 waits
// forever (bounded only by ctx and the connection closing).
func (c *Client) send(ctx context.Context, id string, framed []byte, timeout time.Duration) (resultPacket, error) {
	wait := make(chan resultPacket, 1)
	c.mu.Lock()
	c.pending[id] = wait
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if c.server.Debug {
		c.server.logf("[client %d] <- %s", c.ID, bytes.TrimSuffix(framed, []byte{0}))
	}
	if _, err := c.conn.Write(framed); err != nil {
		c.disconnect()
		return resultPacket{}, fmt.Errorf("write packet: %w", err)
	}

	var timeoutCh <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case result := <-wait:
		return result, nil
	case <-timeoutCh:
		return resultPacket{Outcome: OutcomeTimeout}, nil
	case <-ctx.Done():
		return resultPacket{}, ctx.Err()
	case <-c.done:
		return resultPacket{}, net.ErrClosed
	}
}

func (c *Client) readLoop() {
	defer c.disconnect()

	reader := bufio.NewReader(c.conn)
	for {
		raw, err := reader.ReadBytes(0)
		if err != nil {
			return
		}

		packet := bytes.TrimSuffix(raw, []byte{0})
		if len(packet) == 0 {
			continue
		}
		c.handlePacket(packet)
	}
}

func (c *Client) handlePacket(packet []byte) {
	var env envelope
	if err := json.Unmarshal(packet, &env); err != nil {
		c.server.logf("[client %d] bad packet: %v", c.ID, err)
		return
	}

	if c.server.Debug {
		c.server.logf("[client %d] -> %s", c.ID, packet)
	}

	switch env.Type {
	case PacketTypeResult:
		var result resultPacket
		if err := json.Unmarshal(packet, &result); err != nil {
			return
		}

		c.mu.Lock()
		wait, ok := c.pending[result.ID]
		c.mu.Unlock()
		if ok {
			wait <- result
		}

	case PacketTypeActionEnded:
		var ended actionEndedPacket
		if err := json.Unmarshal(packet, &ended); err != nil {
			return
		}
		c.server.onActionEnded(c, ended.ID, ended.Outcome)

	case PacketTypeHook:
		var hook hookPacket
		if err := json.Unmarshal(packet, &hook); err != nil {
			return
		}
		c.server.onHook(c, hook.Hook)
	}
}

func (c *Client) disconnect() {
	c.closeErr.Do(func() {
		close(c.done)
		c.conn.Close()
		c.server.removeClient(c)
		c.server.logf("[client %d] disconnected", c.ID)
		c.server.onDisconnect(c)
	})
}
