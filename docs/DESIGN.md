# Design Document: Custom Network Proxy Server

## 1. Overview

This document describes the architecture, design decisions, and implementation details of the custom network proxy server. The proxy server is implemented in Go and provides HTTP forwarding, rule-based filtering, logging, and optional features like HTTPS tunneling and caching.

## 2. Architecture

### 2.1 High-Level Architecture

The proxy server follows a layered architecture:

```
┌─────────────────────────────────────────────────┐
│         Client Connections (TCP)                │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│      Listener & Concurrency Layer               │
│  - TCP Listener                                 │
│  - Connection Acceptance                        │
│  - Concurrency Management (Goroutines/Pool)    │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│         Protocol Layer (HTTP Parser)            │
│  - Request Line Parsing                         │
│  - Header Parsing                               │
│  - Body Handling                                │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│         Policy/Control Layer                    │
│  - Filter (Domain/IP Blocking)                  │
│  - Authentication (Optional)                    │
│  - Configuration Management                     │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│         I/O & Data Path Layer                   │
│  - Forwarder (Upstream Connection)              │
│  - Response Streaming                           │
│  - Cache (Optional)                             │
│  - Logger                                        │
└──────────────────┬──────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────┐
│         Upstream Servers                        │
└─────────────────────────────────────────────────┘
```

### 2.2 Component Descriptions

#### 2.2.1 Server/Listener Module (`server.go`)

**Responsibilities:**
- Creates and binds TCP listener socket
- Accepts incoming client connections
- Manages connection lifecycle
- Coordinates with concurrency layer
- Handles graceful shutdown

**Key Functions:**
- `NewServer()`: Initializes server with configuration
- `Start()`: Main accept loop
- `handleConnection()`: Processes individual connections
- `Shutdown()`: Graceful shutdown

**Design Decisions:**
- Uses Go's `net.Listen()` for TCP listening
- Sets socket options (SO_REUSEADDR handled by Go)
- Implements deadline-based accept loop for responsive shutdown

#### 2.2.2 Concurrency Module (`workerpool.go`, `server.go`)

**Responsibilities:**
- Manages concurrent connection handling
- Implements thread-per-connection and thread pool models
- Ensures thread-safe access to shared resources

**Models:**
1. **Thread-per-Connection**: Each connection spawns a goroutine
   - Simple and straightforward
   - Good for educational purposes
   - Resource usage scales with connections

2. **Thread Pool**: Fixed-size worker pool with queue
   - Better resource control
   - Configurable pool size
   - Queue-based connection distribution

**Design Decisions:**
- Go's goroutines provide lightweight concurrency
- Worker pool uses buffered channels for queue
- All shared data structures use mutexes or channel-based synchronization

#### 2.2.3 HTTP Parser Module (`parser.go`)

**Responsibilities:**
- Parses HTTP request line (method, target, version)
- Parses HTTP headers into map structure
- Handles request bodies based on Content-Length
- Extracts destination host and port
- Serializes requests for upstream forwarding

**Key Functions:**
- `ParseHTTPRequest()`: Main parsing function
- `extractHostAndPort()`: Extracts destination from URI or Host header
- `readBody()`: Reads request body
- `SerializeRequest()`: Converts parsed request to bytes

**Design Decisions:**
- Uses `bufio.Reader` for efficient reading
- Supports both absolute-form and origin-form URIs
- Handles CONNECT method specially
- Limits body size to prevent memory exhaustion (10MB default)

**Supported Methods:**
- GET, HEAD, POST (with body)
- CONNECT (for HTTPS tunneling)

#### 2.2.4 Forwarder Module (`forwarder.go`)

**Responsibilities:**
- Establishes TCP connections to upstream servers
- Forwards HTTP requests to upstream
- Streams responses back to clients
- Handles CONNECT tunneling for HTTPS

**Key Functions:**
- `ForwardRequest()`: Main forwarding logic
- `forwardResponse()`: Streams response from upstream
- `HandleCONNECT()`: Bidirectional tunneling
- `streamBody()`: Efficient body streaming

**Design Decisions:**
- Uses streaming to avoid buffering entire responses
- Sets timeouts on upstream connections (30 seconds)
- Handles partial reads/writes correctly
- For CONNECT, uses `io.Copy()` for bidirectional forwarding

#### 2.2.5 Filter Module (`filter.go`)

**Responsibilities:**
- Loads blocking rules from file
- Matches domains and IPs against rules
- Supports exact matching and wildcard subdomain matching
- Thread-safe rule access

**Key Functions:**
- `LoadRules()`: Loads rules from file
- `IsBlocked()`: Checks if host is blocked
- `GetBlockedCount()`: Returns statistics

**Design Decisions:**
- Canonicalizes hostnames (lowercase, trimmed)
- Separate maps for domains and IPs
- Supports wildcard matching (*.example.com)
- Thread-safe with read-write mutex

**Rule Format:**
- One entry per line
- # for comments
- Supports exact domain, IP, and wildcard

#### 2.2.6 Logger Module (`logger.go`)

**Responsibilities:**
- Thread-safe file logging
- Formats log entries with timestamps and metrics
- Handles log rotation based on size

**Key Functions:**
- `NewLogger()`: Initializes logger
- `Log()`: Writes log entry
- `formatLogEntry()`: Formats entry as single line
- `rotate()`: Handles log rotation

**Design Decisions:**
- Uses mutex for thread safety
- Immediate sync after each write (for debugging)
- Size-based rotation with timestamped old files
- ISO 8601 timestamp format

**Log Format:**
```
TIMESTAMP CLIENT_IP:PORT -> DEST_HOST:PORT "METHOD TARGET HTTP/VERSION" ACTION STATUS BYTES_UP BYTES_DOWN [BLOCKED: rule]
```

#### 2.2.7 Cache Module (`cache.go`)

**Responsibilities:**
- LRU cache for HTTP responses
- Cache key generation
- Eviction based on size and entry count
- Thread-safe operations

**Key Functions:**
- `Get()`: Retrieves cached entry
- `Put()`: Stores entry with eviction
- `evictLRU()`: Removes least recently used entry
- `GetStats()`: Returns cache statistics

**Design Decisions:**
- LRU implemented with hash map + access order list
- O(1) average time for get/put operations
- Size-based and count-based eviction
- Currently simplified (does not capture response body in current implementation)

**Limitations:**
- Current implementation does not fully capture response bodies for caching
- Would need to modify forwarder to intercept and cache responses

#### 2.2.8 Configuration Module (`config.go`)

**Responsibilities:**
- Parses configuration file (INI format)
- Validates configuration values
- Provides defaults for missing values
- Supports both JSON and INI formats

**Key Functions:**
- `LoadConfigFromINI()`: Loads INI-style config
- `LoadConfig()`: Loads JSON config
- `Validate()`: Validates configuration

**Design Decisions:**
- Simple key=value format for readability
- Sensible defaults for all optional parameters
- Fail-fast validation with clear error messages
- Supports comments in config file

## 3. Data Flow

### 3.1 Normal HTTP Request Flow

```
1. Client connects to proxy (TCP)
   ↓
2. Server accepts connection
   ↓
3. Connection handler goroutine spawned
   ↓
4. Read HTTP request headers
   ↓
5. Parse request (method, target, headers)
   ↓
6. Extract destination host:port
   ↓
7. Check filter rules
   ├─ Blocked? → Send 403, log, close
   └─ Allowed? → Continue
   ↓
8. Check cache (if enabled)
   ├─ Hit? → Serve from cache, log, close
   └─ Miss? → Continue
   ↓
9. Connect to upstream server
   ↓
10. Forward request to upstream
   ↓
11. Stream response from upstream to client
   ↓
12. Log request metrics
   ↓
13. Close connections
```

### 3.2 CONNECT (HTTPS) Flow

```
1. Client sends CONNECT host:port
   ↓
2. Parse CONNECT request
   ↓
3. Check filter rules
   ├─ Blocked? → Send 403, close
   └─ Allowed? → Continue
   ↓
4. Connect to upstream host:port
   ├─ Failed? → Send 502, close
   └─ Success? → Continue
   ↓
5. Send "200 Connection Established"
   ↓
6. Bidirectional byte forwarding
   ├─ Client → Upstream (goroutine)
   └─ Upstream → Client (goroutine)
   ↓
7. One side closes → Close both connections
```

## 4. Concurrency Model

### 4.1 Thread-per-Connection

**Implementation:**
- Each accepted connection spawns a goroutine
- Goroutine handles entire connection lifecycle
- No shared queue or pool

**Advantages:**
- Simple to implement and understand
- Good for educational purposes
- Natural concurrency model in Go

**Disadvantages:**
- Resource usage scales with connections
- No resource limits

### 4.2 Thread Pool

**Implementation:**
- Fixed-size worker pool (configurable)
- Buffered channel as work queue
- Workers pull connections from queue

**Advantages:**
- Controlled resource usage
- Better for sustained loads
- Configurable pool size

**Disadvantages:**
- More complex implementation
- Queue can fill up under high load

### 4.3 Thread Safety

All shared data structures are protected:
- **Filter**: Read-write mutex for rule access
- **Logger**: Mutex for file writes
- **Cache**: Mutex for all operations
- **Configuration**: Read-only after initialization

## 5. Error Handling

### 5.1 Request Parsing Errors
- Malformed request line → 400 Bad Request
- Missing Host header → 400 Bad Request
- Invalid Content-Length → 400 Bad Request
- Body too large → 400 Bad Request

### 5.2 Upstream Connection Errors
- Connection timeout → 502 Bad Gateway
- DNS failure → 502 Bad Gateway
- Connection refused → 502 Bad Gateway

### 5.3 Filtering
- Blocked domain → 403 Forbidden
- Blocked IP → 403 Forbidden

### 5.4 Server Errors
- Configuration errors → Fail fast with clear message
- Log file errors → Log to stderr
- Socket errors → Log and continue (don't crash)

## 6. Security Considerations

### 6.1 Input Validation
- Request line parsing with bounds checking
- Header size limits
- Body size limits (10MB default)
- Hostname validation

### 6.2 Resource Limits
- Connection timeouts
- Read/write timeouts
- Maximum body size
- Log file size rotation

### 6.3 Memory Safety
- Go's memory safety prevents buffer overflows
- No manual memory management
- Bounded buffers for I/O

### 6.4 Logging Security
- Sanitize log output (though Go's string handling helps)
- No sensitive data in logs (config values, auth tokens)

## 7. Performance Considerations

### 7.1 Streaming
- Responses streamed without full buffering
- Reduces memory usage for large responses
- Enables early response delivery

### 7.2 Concurrency
- Goroutines provide efficient concurrency
- No OS thread overhead per connection
- Efficient channel-based communication

### 7.3 I/O Efficiency
- Buffered reading for headers
- Chunked writing for responses
- Timeout handling prevents hanging connections

### 7.4 Caching (Optional)
- LRU eviction for memory efficiency
- O(1) average time operations
- Size-based limits

## 8. Testing Strategy

### 8.1 Unit Tests
- Parser tests for various request formats
- Filter tests for matching rules
- Configuration validation tests

### 8.2 Integration Tests
- End-to-end request forwarding
- Filtering behavior
- CONNECT tunneling
- Concurrent connection handling

### 8.3 Manual Testing
- Shell scripts for common scenarios
- Load testing with multiple clients
- Error condition testing

## 9. Future Enhancements

### 9.1 Short-term
- Full response caching with body capture
- Connection keep-alive support
- Better error messages

### 9.2 Medium-term
- HTTP/2 support
- WebSocket proxy
- Dynamic rule reloading
- Metrics endpoint

### 9.3 Long-term
- Distributed proxy cluster
- Advanced authentication
- Traffic analysis and reporting
- Plugin system for custom filters

## 10. Conclusion

This proxy server demonstrates core networking and systems programming concepts while maintaining a clean, modular architecture. The use of Go provides excellent concurrency support and memory safety, making it suitable for educational purposes and as a foundation for more advanced features.

The design prioritizes clarity and correctness over performance optimizations, making it easy to understand, modify, and extend.

