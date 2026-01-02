# Custom Network Proxy Server

A user-space HTTP proxy server written in Go that provides rule-based filtering, logging, and transparent request forwarding. This proxy server demonstrates core networking concepts including socket programming, concurrency, HTTP protocol handling, and I/O management.

## Features

### Core Features
- **HTTP Proxy**: Forward HTTP requests to upstream servers
- **Rule-based Filtering**: Block requests to specific domains or IP addresses
- **Comprehensive Logging**: Detailed request/response logging with metrics
- **Concurrency Models**: Thread-per-connection or thread pool support
- **Configuration Management**: Flexible configuration via INI-style config files
- **Graceful Shutdown**: Clean shutdown handling with SIGINT/SIGTERM

### Optional Features
- **HTTPS CONNECT Tunneling**: Support for HTTPS traffic via CONNECT method
- **Response Caching**: LRU cache for HTTP responses (optional)
- **Authentication**: Token-based authentication (optional)

## Project Structure

```
Custom_sever/
├── src/                    # Source code
│   ├── main.go            # Entry point
│   ├── server.go          # Main server implementation
│   ├── config.go          # Configuration management
│   ├── parser.go          # HTTP request parsing
│   ├── forwarder.go       # Upstream forwarding
│   ├── filter.go          # Domain/IP filtering
│   ├── logger.go          # Thread-safe logging
│   ├── cache.go           # LRU caching (optional)
│   └── workerpool.go      # Worker pool implementation
├── config/                # Configuration files
│   ├── proxy.conf         # Server configuration
│   └── blocked_domains.txt # Filter rules
├── tests/                 # Test scripts
│   ├── test_basic.sh      # Basic functionality tests
│   ├── test_blocking.sh   # Filtering tests
│   ├── test_concurrent.sh # Concurrency tests
│   └── test_https.sh      # HTTPS tunneling tests
├── docs/                  # Documentation
│   └── DESIGN.md          # Design document
├── Makefile                # Build and test automation
└── README.md              # This file
```

## Building

### Prerequisites
- Go 1.21 or later
- Make (optional, for using Makefile)

### Build Instructions

```bash
# Using Makefile
make build

# Or manually
go build -o bin/proxy.exe ./src
```

## Configuration

### Server Configuration (`config/proxy.conf`)

```ini
# Network settings
listen_address=0.0.0.0
listen_port=8888

# Concurrency model: thread_per_connection or thread_pool
concurrency_model=thread_per_connection
thread_pool_size=10

# Logging settings
log_file_path=proxy.log
log_max_size_mb=100

# Filtering
blocked_domains_file=config/blocked_domains.txt

# Optional features
enable_caching=false
cache_max_entries=1000
enable_connect_tunneling=true

# Authentication (leave empty to disable)
authentication_token=
```

### Filter Rules (`config/blocked_domains.txt`)

```
# Blocked Domains and IPs
# One entry per line, # for comments

example.com
badsite.org
192.0.2.5

# Wildcard subdomain matching
*.malicious.com
```

## Running

### Start the Proxy Server

```bash
# Using Makefile
make run

# Or manually
./bin/proxy.exe -config config/proxy.conf
```

The server will start listening on the configured address and port (default: `0.0.0.0:8888`).

### Using the Proxy

Configure your HTTP client to use the proxy:

```bash
# Using curl
curl -x localhost:8888 http://example.com

# Using curl with HTTPS (requires CONNECT tunneling)
curl -x localhost:8888 https://example.com

# Using environment variables
export http_proxy=http://localhost:8888
export https_proxy=http://localhost:8888
curl http://example.com
```

## Testing

### Run All Tests

```bash
# Make sure the proxy server is running first
make test
```

### Individual Test Suites

```bash
make test-basic      # Basic functionality
make test-blocking   # Filtering tests
make test-concurrent # Concurrency tests
make test-https      # HTTPS tunneling
```

### Manual Testing

```bash
# Simple GET request
curl -x localhost:8888 http://httpbin.org/get

# HEAD request
curl -x localhost:8888 -I http://httpbin.org/get

# POST request
curl -x localhost:8888 -X POST -d "data=test" http://httpbin.org/post

# HTTPS request
curl -x localhost:8888 https://httpbin.org/get
```

## Logging

The proxy server logs all requests to `proxy.log` (configurable) with the following format:

```
2025-01-01T10:12:34Z 192.0.2.10:54321 -> example.com:80 "GET http://example.com/ HTTP/1.1" ALLOWED 200 1024 8192
```

Log entries include:
- Timestamp (ISO 8601)
- Client IP and port
- Destination host and port
- HTTP method and request target
- Action (ALLOWED, BLOCKED, CACHE_HIT, etc.)
- Upstream status code
- Bytes sent upstream
- Bytes received downstream

## Architecture

### Core Components

1. **Server/Listener**: Accepts TCP connections and manages the accept loop
2. **Concurrency Layer**: Handles connections via goroutines (thread-per-connection) or worker pool
3. **HTTP Parser**: Parses HTTP requests, extracts headers, and handles request bodies
4. **Filter**: Applies blocking rules based on domain/IP matching
5. **Forwarder**: Establishes upstream connections and streams data bidirectionally
6. **Logger**: Thread-safe file logging with rotation support
7. **Cache**: LRU cache for HTTP responses (optional)

### Concurrency Models

- **Thread-per-Connection**: Each client connection is handled by a dedicated goroutine
- **Thread Pool**: Fixed-size worker pool processes connections from a queue

### Data Flow

```
Client → Proxy Listener → Connection Handler → HTTP Parser
                                                      ↓
                                              Filter Check
                                                      ↓
                                              [Blocked?] → 403 Response
                                                      ↓
                                              Forwarder → Upstream Server
                                                      ↓
                                              Response → Client
```

## Design Decisions

- **Go Language**: Chosen for excellent concurrency support (goroutines), standard library networking, and clean code structure
- **Thread-per-Connection Baseline**: Simplest model for educational purposes, easy to understand and debug
- **Streaming Forwarding**: Responses are streamed without buffering entire responses in memory
- **INI-style Config**: Simple, human-readable configuration format
- **Modular Design**: Clear separation of concerns for easy testing and modification

## Limitations

- HTTP/1.1 only (no HTTP/2 or HTTP/3)
- Basic chunked encoding support (transparent forwarding)
- Simplified caching (does not capture response body for caching in current implementation)
- No persistent connection reuse (one request per connection)
- No advanced HTTP features (pipelining, advanced keep-alive)

## Security Considerations

- Input validation for request parsing
- Size limits on request headers and bodies
- Timeout handling to prevent resource exhaustion
- Thread-safe logging and shared data structures
- No buffer overflow vulnerabilities (Go's memory safety)

