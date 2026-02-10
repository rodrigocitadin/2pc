package internal

import (
	"errors"
	"log/slog"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type Peer interface {
	ID() int
	Call(method string, args, reply any) error
	Close() error
}

type peer struct {
	mu      sync.Mutex
	id      int
	address string
	client  *rpc.Client
	logger  *slog.Logger
}

func (p *peer) ID() int {
	return p.id
}

func (p *peer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		p.logger.Info("Closing peer connection")
		return p.client.Close()
	}
	return nil
}

func (p *peer) Call(method string, args any, reply any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client == nil {
		p.logger.Debug("Dialing peer")
		conn, err := net.DialTimeout("tcp", p.address, 2*time.Second)
		if err != nil {
			p.logger.Warn("Failed to dial peer", "error", err)
			return err
		}
		p.client = rpc.NewClient(conn)
	}

	call := p.client.Go(method, args, reply, nil)

	select {
	case <-call.Done:
		if call.Error == rpc.ErrShutdown {
			p.logger.Warn("RPC connection shutdown, resetting client")
			p.client.Close()
			p.client = nil
		}
		return call.Error

	case <-time.After(5 * time.Second):
		p.logger.Warn("RPC call timed out, closing connection")
		p.client.Close()
		p.client = nil
		return errors.New("rpc call timed out")
	}
}

func NewPeer(id int, address string, logger *slog.Logger) Peer {
	return &peer{
		id:      id,
		address: address,
		logger:  logger.With("peer_scope", "client", "target_peer_id", id),
	}
}
