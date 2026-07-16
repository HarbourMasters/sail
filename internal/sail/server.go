package sail

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

// Server accepts TCP connections from the game and tracks connected clients.
// Set OnConnect/OnDisconnect/OnHook before calling Start.
type Server struct {
	Debug bool

	OnConnect     func(*Client)
	OnDisconnect  func(*Client)
	OnHook        func(client *Client, hook Hook)
	OnActionEnded func(client *Client, id string, outcome Outcome)
	OnAcceptError func(error)

	mu       sync.Mutex
	listener net.Listener
	clients  map[int]*Client
	nextID   int
}

// NewServer creates a Server that isn't listening yet.
func NewServer() *Server {
	return &Server{clients: make(map[int]*Client)}
}

// onDisconnect/onHook/onActionEnded are nil-guarded wrappers Client calls from
// its read goroutine.
func (s *Server) onDisconnect(c *Client) {
	if s.OnDisconnect != nil {
		s.OnDisconnect(c)
	}
}

func (s *Server) onHook(c *Client, hook Hook) {
	if s.OnHook != nil {
		s.OnHook(c, hook)
	}
}

func (s *Server) onActionEnded(c *Client, id string, outcome Outcome) {
	if s.OnActionEnded != nil {
		s.OnActionEnded(c, id, outcome)
	}
}

// Start binds port and accepts connections in the background, returning once
// bound so a port conflict surfaces immediately. Accept errors (other than
// Stop closing the listener) go to OnAcceptError.
func (s *Server) Start(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", port, err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	go s.acceptLoop(listener)
	return nil
}

func (s *Server) acceptLoop(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) && s.OnAcceptError != nil {
				s.OnAcceptError(err)
			}
			return
		}

		s.mu.Lock()
		s.nextID++
		id := s.nextID
		client := newClient(id, conn, s)
		s.clients[id] = client
		s.mu.Unlock()

		s.logf("client %d connected from %s", id, client.RemoteAddr())
		if s.OnConnect != nil {
			s.OnConnect(client)
		}
	}
}

// Stop closes the listener and disconnects all currently connected clients.
func (s *Server) Stop() error {
	for _, c := range s.Clients() {
		c.disconnect()
	}

	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()

	if listener == nil {
		return nil
	}
	return listener.Close()
}

// Clients returns the currently connected game clients.
func (s *Server) Clients() []*Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	clients := make([]*Client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	return clients
}

// ClientCount returns the connected client count without allocating (unlike
// Clients).
func (s *Server) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

func (s *Server) removeClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c.ID)
}

func (s *Server) logf(format string, args ...any) {
	if s.Debug {
		log.Printf("[sail] "+format, args...)
	}
}
