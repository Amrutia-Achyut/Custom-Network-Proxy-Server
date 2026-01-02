package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// HTTPRequest represents a parsed HTTP request
type HTTPRequest struct {
	Method        string
	RequestTarget string
	Version       string
	Headers       map[string]string
	Body          []byte
	Host          string
	Port          int
	IsConnect     bool
}

// ParseHTTPRequest parses an HTTP request from a reader
func ParseHTTPRequest(reader *bufio.Reader) (*HTTPRequest, error) {
	req := &HTTPRequest{
		Headers: make(map[string]string),
	}

	// Read request line
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read request line: %w", err)
	}

	requestLine = strings.TrimRight(requestLine, "\r\n")
	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid request line: %s", requestLine)
	}

	req.Method = strings.ToUpper(parts[0])
	req.RequestTarget = parts[1]
	req.Version = parts[2]

	// Check if it's a CONNECT request
	if req.Method == "CONNECT" {
		req.IsConnect = true
		// For CONNECT, target is host:port
		hostPort := strings.SplitN(req.RequestTarget, ":", 2)
		if len(hostPort) != 2 {
			return nil, fmt.Errorf("invalid CONNECT target: %s", req.RequestTarget)
		}
		req.Host = hostPort[0]
		port, err := strconv.Atoi(hostPort[1])
		if err != nil {
			return nil, fmt.Errorf("invalid port in CONNECT: %w", err)
		}
		req.Port = port
		return req, nil
	}

	// Read headers until empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read headers: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // End of headers
		}

		// Parse header
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue // Invalid header, skip
		}

		key := strings.TrimSpace(strings.ToLower(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		req.Headers[key] = value
	}

	// Extract host and port from request
	if err := req.extractHostAndPort(); err != nil {
		return nil, err
	}

	// Read body if present
	if err := req.readBody(reader); err != nil {
		return nil, err
	}

	return req, nil
}

// extractHostAndPort extracts host and port from request target and headers
func (req *HTTPRequest) extractHostAndPort() error {
	// Try absolute-form URI first
	if strings.HasPrefix(req.RequestTarget, "http://") || strings.HasPrefix(req.RequestTarget, "https://") {
		parsedURL, err := url.Parse(req.RequestTarget)
		if err != nil {
			return fmt.Errorf("failed to parse absolute URI: %w", err)
		}
		req.Host = parsedURL.Hostname()
		portStr := parsedURL.Port()
		if portStr == "" {
			if strings.HasPrefix(req.RequestTarget, "https://") {
				req.Port = 443
			} else {
				req.Port = 80
			}
		} else {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("invalid port in URI: %w", err)
			}
			req.Port = port
		}
		return nil
	}

	// Try Host header
	hostHeader, ok := req.Headers["host"]
	if !ok {
		return fmt.Errorf("missing Host header")
	}

	// Parse host:port from Host header
	hostPort := strings.SplitN(hostHeader, ":", 2)
	req.Host = hostPort[0]
	if len(hostPort) == 2 {
		port, err := strconv.Atoi(hostPort[1])
		if err != nil {
			return fmt.Errorf("invalid port in Host header: %w", err)
		}
		req.Port = port
	} else {
		req.Port = 80 // Default HTTP port
	}

	return nil
}

// readBody reads the request body if present
func (req *HTTPRequest) readBody(reader *bufio.Reader) error {
	contentLengthStr, ok := req.Headers["content-length"]
	if !ok {
		return nil // No body
	}

	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return fmt.Errorf("invalid Content-Length: %w", err)
	}

	if contentLength < 0 {
		return fmt.Errorf("negative Content-Length")
	}

	if contentLength > 10*1024*1024 { // 10MB limit
		return fmt.Errorf("Content-Length too large: %d", contentLength)
	}

	req.Body = make([]byte, contentLength)
	_, err = io.ReadFull(reader, req.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	return nil
}

// SerializeRequest serializes the request for forwarding to upstream
func (req *HTTPRequest) SerializeRequest() []byte {
	var builder strings.Builder

	// Request line
	if strings.HasPrefix(req.RequestTarget, "http://") || strings.HasPrefix(req.RequestTarget, "https://") {
		// Convert absolute URI to origin-form for upstream
		parsedURL, err := url.Parse(req.RequestTarget)
		if err == nil {
			path := parsedURL.Path
			if path == "" {
				path = "/"
			}
			if parsedURL.RawQuery != "" {
				path += "?" + parsedURL.RawQuery
			}
			builder.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, path, req.Version))
		} else {
			builder.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, req.RequestTarget, req.Version))
		}
	} else {
		builder.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, req.RequestTarget, req.Version))
	}

	// Headers
	for key, value := range req.Headers {
		// Capitalize header name properly
		headerName := capitalizeHeader(key)
		builder.WriteString(fmt.Sprintf("%s: %s\r\n", headerName, value))
	}
	builder.WriteString("\r\n")

	// Body
	if len(req.Body) > 0 {
		builder.Write(req.Body)
	}

	return []byte(builder.String())
}

// capitalizeHeader capitalizes HTTP header names (e.g., "content-type" -> "Content-Type")
func capitalizeHeader(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "-")
}

// GetClientIP extracts the client IP from a connection
func GetClientIP(conn net.Conn) string {
	addr := conn.RemoteAddr()
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}
	return addr.String()
}

