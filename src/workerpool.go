package main

import (
	"net"
	"sync"
)

// WorkerPool manages a pool of worker goroutines
type WorkerPool struct {
	size        int
	workQueue   chan net.Conn
	handler     func(net.Conn)
	wg          sync.WaitGroup
	shutdown    chan struct{}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(size int, handler func(net.Conn)) *WorkerPool {
	return &WorkerPool{
		size:      size,
		workQueue: make(chan net.Conn, size*2), // Buffer queue
		handler:   handler,
		shutdown:  make(chan struct{}),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.size; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker is the worker goroutine
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.shutdown:
			return
		case conn := <-wp.workQueue:
			wp.handler(conn)
		}
	}
}

// Submit submits a connection to the work queue
func (wp *WorkerPool) Submit(conn net.Conn) {
	select {
	case wp.workQueue <- conn:
	default:
		// Queue full, close connection
		conn.Close()
	}
}

// Shutdown shuts down the worker pool
func (wp *WorkerPool) Shutdown() {
	close(wp.shutdown)
	close(wp.workQueue)
	wp.wg.Wait()
}

