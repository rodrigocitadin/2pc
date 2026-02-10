package internal

import (
	"errors"
	"time"
)

type Result[T any] struct {
	PeerID int
	Value  T
	Err    error
}

func Broadcast[T any](peers []Peer, method string, args any) []Result[T] {
	resultsChan := make(chan Result[T], len(peers))

	for _, p := range peers {
		go func(peer Peer) {
			done := make(chan error, 1)
			var reply T

			go func() {
				done <- peer.Call(method, args, &reply)
			}()

			select {
			case err := <-done:
				resultsChan <- Result[T]{PeerID: peer.ID(), Value: reply, Err: err}
			case <-time.After(5 * time.Second): // Hard timeout
				resultsChan <- Result[T]{PeerID: peer.ID(), Err: errors.New("timeout")}
			}
		}(p)
	}

	results := make([]Result[T], 0, len(peers))
	for range peers {
		results = append(results, <-resultsChan)
	}

	return results
}
