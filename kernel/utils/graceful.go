package utils

import (
	"context"
	"sync"
	"time"
)

// GracefulShutdown manages graceful shutdown of components
type GracefulShutdown struct {
	mu         sync.Mutex
	shutdownFn []func() error
	timeout    time.Duration
	logger     *Logger
}

// NewGracefulShutdown creates a new graceful shutdown manager
func NewGracefulShutdown(timeout time.Duration, logger *Logger) *GracefulShutdown {
	if logger == nil {
		logger = DefaultLogger("shutdown")
	}

	return &GracefulShutdown{
		shutdownFn: make([]func() error, 0),
		timeout:    timeout,
		logger:     logger,
	}
}

// Register registers a shutdown function
func (g *GracefulShutdown) Register(fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.shutdownFn = append(g.shutdownFn, fn)
}

// Shutdown executes all registered shutdown functions
func (g *GracefulShutdown) Shutdown(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.logger.Info("Starting graceful shutdown",
		Int("components", len(g.shutdownFn)),
	)

	// Create timeout context
	shutdownCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	// Execute shutdown functions in reverse order (LIFO)
	errChan := make(chan error, len(g.shutdownFn))
	var wg sync.WaitGroup

	for i := len(g.shutdownFn) - 1; i >= 0; i-- {
		wg.Add(1)
		fn := g.shutdownFn[i]

		go func(idx int, shutdownFn func() error) {
			defer wg.Done()

			if err := shutdownFn(); err != nil {
				g.logger.Error("Shutdown function failed",
					Int("index", idx),
					Err(err),
				)
				errChan <- err
			}
		}(i, fn)
	}

	// Wait for all shutdown functions or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Graceful shutdown complete")
		return nil
	case <-shutdownCtx.Done():
		g.logger.Warn("Graceful shutdown timed out")
		return NewError("shutdown timeout")
	}
}
