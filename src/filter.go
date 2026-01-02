package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// Filter manages blocked domains and IPs
type Filter struct {
	blockedDomains map[string]bool
	blockedIPs     map[string]bool
	mu             sync.RWMutex
}

// NewFilter creates a new filter instance
func NewFilter() *Filter {
	return &Filter{
		blockedDomains: make(map[string]bool),
		blockedIPs:     make(map[string]bool),
	}
}

// LoadRules loads blocking rules from a file
func (f *Filter) LoadRules(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty rules
			return nil
		}
		return fmt.Errorf("failed to open filter file: %w", err)
	}
	defer file.Close()

	f.mu.Lock()
	defer f.mu.Unlock()

	// Clear existing rules
	f.blockedDomains = make(map[string]bool)
	f.blockedIPs = make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		// Canonicalize: lowercase and trim
		line = strings.ToLower(strings.TrimSpace(line))

		// Check if it's an IP address
		if ip := net.ParseIP(line); ip != nil {
			f.blockedIPs[line] = true
		} else {
			// It's a domain
			f.blockedDomains[line] = true
		}
	}

	return scanner.Err()
}

// IsBlocked checks if a hostname or IP is blocked
func (f *Filter) IsBlocked(host string) (bool, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Canonicalize hostname
	host = strings.ToLower(strings.TrimSpace(host))

	// Check exact domain match
	if f.blockedDomains[host] {
		return true, host
	}

	// Check IP match
	if f.blockedIPs[host] {
		return true, host
	}

	// Check suffix matching (e.g., *.example.com)
	for domain := range f.blockedDomains {
		if strings.HasPrefix(domain, "*.") {
			suffix := domain[2:] // Remove "*."
			if strings.HasSuffix(host, "."+suffix) || host == suffix {
				return true, domain
			}
		}
	}

	return false, ""
}

// GetBlockedCount returns the number of blocked rules
func (f *Filter) GetBlockedCount() (int, int) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.blockedDomains), len(f.blockedIPs)
}

