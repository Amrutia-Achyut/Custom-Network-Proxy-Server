package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	upstreamTimeout = 30 * time.Second
	readBufferSize  = 8192
)

// Forwarder handles forwarding requests to upstream servers
type Forwarder struct {
	config *Config
}

// NewForwarder creates a new forwarder instance
func NewForwarder(config *Config) *Forwarder {
	return &Forwarder{
		config: config,
	}
}

// ForwardRequest forwards an HTTP request to the upstream server
func (f *Forwarder) ForwardRequest(req *HTTPRequest, clientConn net.Conn) (int, int64, int64, error) {
	// Connect to upstream server
	upstreamAddr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, upstreamTimeout)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to connect to upstream: %w", err)
	}
	defer upstreamConn.Close()

	// Set timeouts
	upstreamConn.SetDeadline(time.Now().Add(upstreamTimeout))

	// Serialize and send request
	requestBytes := req.SerializeRequest()
	bytesUpstream, err := f.writeAll(upstreamConn, requestBytes)
	if err != nil {
		return 0, bytesUpstream, 0, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response from upstream
	statusCode, bytesDownstream, err := f.forwardResponse(upstreamConn, clientConn)
	if err != nil {
		return statusCode, bytesUpstream, bytesDownstream, fmt.Errorf("failed to forward response: %w", err)
	}

	return statusCode, bytesUpstream, bytesDownstream, nil
}

// forwardResponse reads response from upstream and forwards to client
func (f *Forwarder) forwardResponse(upstreamConn net.Conn, clientConn net.Conn) (int, int64, error) {
	reader := bufio.NewReader(upstreamConn)
	
	// Read status line
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read status line: %w", err)
	}

	// Parse status code
	parts := strings.SplitN(strings.TrimSpace(statusLine), " ", 3)
	statusCode := 0
	if len(parts) >= 2 {
		if code, err := strconv.Atoi(parts[1]); err == nil {
			statusCode = code
		}
	}

	// Write status line to client
	bytesWritten, err := f.writeAll(clientConn, []byte(statusLine))
	if err != nil {
		return statusCode, bytesWritten, err
	}

	// Read and forward headers
	headersEnded := false
	for !headersEnded {
		line, err := reader.ReadString('\n')
		if err != nil {
			return statusCode, bytesWritten, fmt.Errorf("failed to read headers: %w", err)
		}

		written, err := f.writeAll(clientConn, []byte(line))
		bytesWritten += written
		if err != nil {
			return statusCode, bytesWritten, err
		}

		// Check for end of headers
		if line == "\r\n" || line == "\n" {
			headersEnded = true
		}
	}

	// Stream body
	bodyBytes, err := f.streamBody(reader, clientConn)
	bytesWritten += bodyBytes
	if err != nil && err != io.EOF {
		return statusCode, bytesWritten, err
	}

	return statusCode, bytesWritten, nil
}

// streamBody streams the response body from upstream to client
func (f *Forwarder) streamBody(reader *bufio.Reader, clientConn net.Conn) (int64, error) {
	var totalBytes int64
	buffer := make([]byte, readBufferSize)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			written, writeErr := f.writeAll(clientConn, buffer[:n])
			totalBytes += written
			if writeErr != nil {
				return totalBytes, writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				return totalBytes, nil
			}
			return totalBytes, err
		}
	}
}

// writeAll writes all bytes, handling partial writes
func (f *Forwarder) writeAll(conn net.Conn, data []byte) (int64, error) {
	var totalWritten int64
	for totalWritten < int64(len(data)) {
		n, err := conn.Write(data[totalWritten:])
		if err != nil {
			return totalWritten, err
		}
		totalWritten += int64(n)
	}
	return totalWritten, nil
}

// HandleCONNECT handles CONNECT tunneling for HTTPS
func (f *Forwarder) HandleCONNECT(req *HTTPRequest, clientConn net.Conn) error {
	// Connect to upstream
	upstreamAddr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, upstreamTimeout)
	if err != nil {
		// Send error response
		response := "HTTP/1.1 502 Bad Gateway\r\n\r\n"
		clientConn.Write([]byte(response))
		return fmt.Errorf("failed to connect to upstream: %w", err)
	}
	defer upstreamConn.Close()

	// Send success response
	response := "HTTP/1.1 200 Connection Established\r\n\r\n"
	if _, err := clientConn.Write([]byte(response)); err != nil {
		return fmt.Errorf("failed to send CONNECT response: %w", err)
	}

	// Bidirectional forwarding
	done := make(chan error, 2)

	// Forward client -> upstream
	go func() {
		_, err := io.Copy(upstreamConn, clientConn)
		done <- err
	}()

	// Forward upstream -> client
	go func() {
		_, err := io.Copy(clientConn, upstreamConn)
		done <- err
	}()

	// Wait for one direction to close
	err = <-done
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

