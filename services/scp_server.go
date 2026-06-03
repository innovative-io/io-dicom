package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"
)

// scpBufSize is the read/write buffer size per connection. A larger value
// reduces syscall overhead when transferring large pixel-data payloads.
const scpBufSize = 256 * 1024

// cancelPollWindow bounds how long SCP waits while polling for interleaved
// DIMSE commands, including in-flight C-CANCEL, during active operations.
const cancelPollWindow = 5 * time.Millisecond

// cancelGraceWindow bounds how long SCP waits for a handler to exit after a
// matching C-CANCEL is received before aborting the association.
const cancelGraceWindow = 250 * time.Millisecond

func (s *scp) Start(ctx context.Context) error {
	var err error
	if s.tlsConfig != nil {
		s.listener, err = tls.Listen("tcp", fmt.Sprintf(":%d", s.Port), s.tlsConfig)
	} else {
		s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	}
	if err != nil {
		return err
	}

	// Close the listener when ctx is cancelled so Accept() unblocks.
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	// When a concurrency limit is configured, sem is a counting semaphore that
	// caps the number of in-flight association goroutines. Acquiring before
	// Accept returns the next connection applies TCP backpressure on a flood
	// instead of spawning unbounded goroutines. A nil sem means unlimited.
	var sem chan struct{}
	if s.maxAssociations > 0 {
		sem = make(chan struct{}, s.maxAssociations)
	}

	for {
		if sem != nil {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return nil
			}
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if sem != nil {
				<-sem
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.logger.Error("accept failed", "error", err)
			return err
		}
		s.logger.Debug("new connection", "remote_addr", conn.RemoteAddr().String())
		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			if sem != nil {
				defer func() { <-sem }()
			}
			s.handleConnection(ctx, c)
		}(conn)
	}
}

func (s *scp) Stop() error {
	if s.listener == nil {
		return nil
	}
	err := s.listener.Close()
	s.wg.Wait()
	return err
}
