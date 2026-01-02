package main

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"
)

// Server represents the proxy server
type Server struct {
	config     *Config
	filter     *Filter
	logger     *Logger
	forwarder  *Forwarder
	cache      *Cache
	listener   net.Listener
	wg         sync.WaitGroup
	shutdown   chan struct{}
	workerPool *WorkerPool
}

// NewServer creates a new server instance
func NewServer(config *Config) (*Server, error) {
	// Load filter rules
	filter := NewFilter()
	if err := filter.LoadRules(config.BlockedDomainsFile); err != nil {
		return nil, fmt.Errorf("failed to load filter rules: %w", err)
	}

	// Initialize logger
	logger, err := NewLogger(config.LogFilePath, config.LogMaxSizeMB)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize forwarder
	forwarder := NewForwarder(config)

	// Initialize cache if enabled
	var cache *Cache
	if config.EnableCaching {
		cache = NewCache(config.CacheMaxEntries)
	}

	server := &Server{
		config:    config,
		filter:    filter,
		logger:    logger,
		forwarder: forwarder,
		cache:     cache,
		shutdown:  make(chan struct{}),
	}

	// Initialize worker pool if using thread pool model
	if config.ConcurrencyModel == "thread_pool" {
		server.workerPool = NewWorkerPool(config.ThreadPoolSize, server.handleConnection)
	}

	return server, nil
}

// Start starts the proxy server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.ListenAddress, s.config.ListenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener

	fmt.Printf("Proxy server listening on %s\n", addr)

	// Start worker pool if applicable
	if s.workerPool != nil {
		s.workerPool.Start()
	}

	// Accept loop
	for {
		select {
		case <-s.shutdown:
			return nil
		default:
			// Set deadline for accept to allow checking shutdown
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))
			conn, err := s.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout, check shutdown again
				}
				return fmt.Errorf("failed to accept connection: %w", err)
			}

			// Handle connection based on concurrency model
			if s.config.ConcurrencyModel == "thread_per_connection" {
				s.wg.Add(1)
				go s.handleConnection(conn)
			} else if s.config.ConcurrencyModel == "thread_pool" {
				s.workerPool.Submit(conn)
			}
		}
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	if s.config.ConcurrencyModel == "thread_per_connection" {
		defer s.wg.Done()
	}

	clientIP := GetClientIP(conn)
	clientPort := 0
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		clientPort = tcpAddr.Port
	}

	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Parse request
	reader := bufio.NewReader(conn)
	req, err := ParseHTTPRequest(reader)
	if err != nil {
		s.sendErrorResponse(conn, 400, "Bad Request")
		s.logRequest(clientIP, clientPort, "", 0, "UNKNOWN", "", "ERROR", 400, 0, 0, err.Error())
		return
	}

	// Check authentication if enabled
	if s.config.AuthToken != "" {
		authHeader := req.Headers["proxy-authorization"]
		if authHeader != s.config.AuthToken {
			s.sendErrorResponse(conn, 407, "Proxy Authentication Required")
			s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "AUTH_FAILED", 407, 0, 0, "")
			return
		}
	}

	// Handle CONNECT for HTTPS tunneling
	if req.IsConnect {
		if !s.config.EnableConnectTunnel {
			s.sendErrorResponse(conn, 501, "Not Implemented")
			s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "BLOCKED", 501, 0, 0, "CONNECT not enabled")
			return
		}

		// Check if blocked
		blocked, rule := s.filter.IsBlocked(req.Host)
		if blocked {
			s.sendErrorResponse(conn, 403, "Forbidden")
			s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "BLOCKED", 403, 0, 0, rule)
			return
		}

		// Handle CONNECT tunneling
		err := s.forwarder.HandleCONNECT(req, conn)
		if err != nil {
			s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "ERROR", 0, 0, 0, err.Error())
		} else {
			s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "ALLOWED", 200, 0, 0, "")
		}
		return
	}

	// Check if blocked
	blocked, rule := s.filter.IsBlocked(req.Host)
	if blocked {
		s.sendErrorResponse(conn, 403, "Forbidden")
		s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "BLOCKED", 403, 0, 0, rule)
		return
	}

	// Check cache for GET requests
	cacheKey := MakeCacheKey(req.Method, req.RequestTarget)
	var statusCode int
	var bytesUpstream, bytesDownstream int64

	if s.cache != nil && cacheKey != "" {
		if cachedEntry, found := s.cache.Get(cacheKey); found {
			// Serve from cache
			s.serveCachedResponse(conn, cachedEntry)
			s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "CACHE_HIT", cachedEntry.StatusCode, 0, int64(len(cachedEntry.Body)), "")
			return
		}
	}

	// Forward request
	statusCode, bytesUpstream, bytesDownstream, err = s.forwarder.ForwardRequest(req, conn)
	if err != nil {
		s.sendErrorResponse(conn, 502, "Bad Gateway")
		s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "ERROR", 502, bytesUpstream, bytesDownstream, err.Error())
		return
	}

	// Cache response if applicable
	if s.cache != nil && IsCacheable(req.Method, statusCode) && cacheKey != "" {
		// Note: In a full implementation, we'd need to capture the response
		// For now, we'll skip caching the response body as it's already been sent
		// This is a simplified version
	}

	s.logRequest(clientIP, clientPort, req.Host, req.Port, req.Method, req.RequestTarget, "ALLOWED", statusCode, bytesUpstream, bytesDownstream, "")
}

// serveCachedResponse serves a response from cache
func (s *Server) serveCachedResponse(conn net.Conn, entry *CacheEntry) {
	// Write status line
	statusLine := fmt.Sprintf("HTTP/1.1 %d OK\r\n", entry.StatusCode)
	conn.Write([]byte(statusLine))

	// Write headers
	for key, value := range entry.Headers {
		headerLine := fmt.Sprintf("%s: %s\r\n", key, value)
		conn.Write([]byte(headerLine))
	}
	conn.Write([]byte("\r\n"))

	// Write body
	conn.Write(entry.Body)
}

// sendErrorResponse sends an HTTP error response
func (s *Server) sendErrorResponse(conn net.Conn, statusCode int, message string) {
	body := fmt.Sprintf("%d %s", statusCode, message)
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, message)
	response += "Content-Type: text/plain\r\n"
	response += fmt.Sprintf("Content-Length: %d\r\n", len(body))
	response += "Connection: close\r\n"
	response += "\r\n"
	response += body

	conn.Write([]byte(response))
}

// logRequest logs a request
func (s *Server) logRequest(clientIP string, clientPort int, destHost string, destPort int, method, target, action string, statusCode int, bytesUp, bytesDown int64, blockedRule string) {
	entry := LogEntry{
		Timestamp:       time.Now(),
		ClientIP:        clientIP,
		ClientPort:      clientPort,
		DestinationHost: destHost,
		DestinationPort: destPort,
		Method:          method,
		RequestTarget:   target,
		Action:          action,
		UpstreamStatus:  statusCode,
		BytesUpstream:   bytesUp,
		BytesDownstream: bytesDown,
		BlockedRule:     blockedRule,
	}
	s.logger.Log(entry)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	fmt.Println("Shutting down server...")
	close(s.shutdown)
	
	if s.listener != nil {
		s.listener.Close()
	}

	if s.workerPool != nil {
		s.workerPool.Shutdown()
	}

	// Wait for active connections
	s.wg.Wait()

	// Close logger
	s.logger.Close()

	fmt.Println("Server shut down complete")
}

