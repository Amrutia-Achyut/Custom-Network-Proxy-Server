package main

import (
	"sync"
	"time"
)

// CacheEntry represents a cached HTTP response
type CacheEntry struct {
	Headers      map[string]string
	StatusCode   int
	Body         []byte
	LastAccessed time.Time
	Size         int64
}

// Cache provides LRU caching for HTTP responses
type Cache struct {
	entries      map[string]*CacheEntry
	accessOrder  []string // LRU list
	maxEntries   int
	maxSize      int64 // Maximum total size in bytes
	currentSize  int64
	mu           sync.RWMutex
}

// NewCache creates a new cache instance
func NewCache(maxEntries int) *Cache {
	return &Cache{
		entries:     make(map[string]*CacheEntry),
		accessOrder: make([]string, 0),
		maxEntries:  maxEntries,
		maxSize:     100 * 1024 * 1024, // 100MB default
	}
}

// Get retrieves a cached response
func (c *Cache) Get(key string) (*CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Update access time and move to end of LRU list
	entry.LastAccessed = time.Now()
	c.moveToEnd(key)

	return entry, true
}

// Put stores a response in the cache
func (c *Cache) Put(key string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate entry size
	entrySize := int64(len(entry.Body))
	for k, v := range entry.Headers {
		entrySize += int64(len(k) + len(v))
	}
	entry.Size = entrySize
	entry.LastAccessed = time.Now()

	// Check if key already exists
	if existing, exists := c.entries[key]; exists {
		c.currentSize -= existing.Size
		c.removeFromOrder(key)
	}

	// Evict if necessary
	for (len(c.entries) >= c.maxEntries || c.currentSize+entrySize > c.maxSize) && len(c.entries) > 0 {
		c.evictLRU()
	}

	// Add new entry
	c.entries[key] = entry
	c.currentSize += entrySize
	c.accessOrder = append(c.accessOrder, key)
}

// moveToEnd moves a key to the end of the access order list
func (c *Cache) moveToEnd(key string) {
	// Remove from current position
	c.removeFromOrder(key)
	// Add to end
	c.accessOrder = append(c.accessOrder, key)
}

// removeFromOrder removes a key from the access order list
func (c *Cache) removeFromOrder(key string) {
	for i, k := range c.accessOrder {
		if k == key {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}
}

// evictLRU evicts the least recently used entry
func (c *Cache) evictLRU() {
	if len(c.accessOrder) == 0 {
		return
	}

	// Remove first (oldest) entry
	key := c.accessOrder[0]
	c.accessOrder = c.accessOrder[1:]

	if entry, exists := c.entries[key]; exists {
		c.currentSize -= entry.Size
		delete(c.entries, key)
	}
}

// Clear clears all cache entries
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
	c.accessOrder = make([]string, 0)
	c.currentSize = 0
}

// GetStats returns cache statistics
func (c *Cache) GetStats() (int, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries), c.currentSize
}

// MakeCacheKey creates a cache key from request method and URI
func MakeCacheKey(method, requestTarget string) string {
	// Only cache GET requests
	if method != "GET" {
		return ""
	}
	// Normalize the URI (could be enhanced to handle query params, etc.)
	return method + ":" + requestTarget
}

// IsCacheable checks if a response can be cached
func IsCacheable(method string, statusCode int) bool {
	// Only cache successful GET requests
	if method != "GET" {
		return false
	}
	// Cache 200 OK responses
	return statusCode == 200
}

