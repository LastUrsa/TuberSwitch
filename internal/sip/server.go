package sip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultHost = "127.0.0.1"
	firstPort   = 47040
	lastPort    = 47049
)

type Server struct {
	service *Service

	mu         sync.Mutex
	httpServer *http.Server
	listener   net.Listener
	address    string
}

func NewServer(service *Service) *Server {
	return &Server{service: service}
}

func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpServer != nil {
		return nil
	}
	if s.service == nil {
		return fmt.Errorf("sip service is required")
	}

	listener, err := listenOnReservedPort()
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:           s.service.Handler(),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	s.listener = listener
	s.httpServer = server
	s.address = "http://" + listener.Addr().String()

	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Listener failures are returned synchronously during startup.
		}
	}()

	if ctx != nil {
		go func() {
			<-ctx.Done()
			_ = s.Stop(context.Background())
		}()
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.httpServer
	s.httpServer = nil
	s.listener = nil
	s.address = ""
	s.mu.Unlock()

	if server == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func (s *Server) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.address
}

func listenOnReservedPort() (net.Listener, error) {
	var lastErr error
	for port := firstPort; port <= lastPort; port++ {
		address := fmt.Sprintf("%s:%d", defaultHost, port)
		listener, err := net.Listen("tcp", address)
		if err == nil {
			return listener, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("listen on TuberSwitch SIP port range: %w", lastErr)
	}
	return nil, fmt.Errorf("listen on TuberSwitch SIP port range")
}
