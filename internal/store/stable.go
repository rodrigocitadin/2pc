package store

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/uuid"
)

type StableStore interface {
	WritePrepared(txID uuid.UUID, value, senderID int) error
	WriteCommited(txID uuid.UUID, value, senderID int) error
	WriteAborted(txID uuid.UUID, senderID int) error
	RecoverLastState() (*entry, error)
	Truncate() error
}

type stableStore struct {
	mu      sync.Mutex
	nodeID  int
	file    *os.File
	encoder *gob.Encoder
}

func (s *stableStore) RecoverLastState() (*entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastState entry
	decoder := gob.NewDecoder(s.file)

	for {
		var current entry
		if err := decoder.Decode(&current); err != nil {
			if err == io.EOF {
				break
			}
			return &lastState, fmt.Errorf("potential corruption at end of log: %w", err)
		}
		lastState = current
	}

	return &lastState, nil
}

func (s *stableStore) Truncate() error {
	return s.file.Truncate(0)
}

func (s *stableStore) WriteAborted(txID uuid.UUID, senderID int) error {
	return s.writeLog(entry{
		TxID:  txID,
		State: TRANSACTION_ABORTED,
	})
}

func (s *stableStore) WriteCommited(txID uuid.UUID, value, senderID int) error {
	return s.writeLog(entry{
		TxID:  txID,
		Value: value,
		State: TRANSACTION_COMMITTED,
	})
}

func (s *stableStore) WritePrepared(txID uuid.UUID, value, senderID int) error {
	return s.writeLog(entry{
		TxID:  txID,
		Value: value,
		State: TRANSACTION_PREPARED,
	})
}

func (s *stableStore) writeLog(entry entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.encoder.Encode(entry); err != nil {
		return err
	}

	return s.file.Sync()
}

func (s *stableStore) Close() error {
	return s.file.Close()
}

func NewStableStore(nodeID int) (StableStore, error) {
	dir := "./logs"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	filename := fmt.Sprintf("%s/node_%d.wal", dir, nodeID)

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &stableStore{
		file:    f,
		encoder: gob.NewEncoder(f),
		nodeID:  nodeID,
	}, nil
}
