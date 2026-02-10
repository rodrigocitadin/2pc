package store

import (
	"errors"
	"maps"
	"sync"

	"github.com/google/uuid"
)

type VolatileStore interface {
	Prepare(txID uuid.UUID, newState int) error
	Commit(txID uuid.UUID) error
	Abort(txID uuid.UUID) error
	Recover(state int, commitedLog map[uuid.UUID]bool)
	State() int
	GetCommittedHistory() map[uuid.UUID]bool
}

type volatileStore struct {
	mu            sync.RWMutex
	locked        bool
	lockedByTx    uuid.UUID
	state         int
	proposedValue int
	committedLog  map[uuid.UUID]bool
}

func (vs *volatileStore) Recover(state int, committedLog map[uuid.UUID]bool) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.state = state
	if committedLog != nil {
		vs.committedLog = committedLog
	} else {
		vs.committedLog = make(map[uuid.UUID]bool)
	}
}

func (vs *volatileStore) GetCommittedHistory() map[uuid.UUID]bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	copyMap := make(map[uuid.UUID]bool, len(vs.committedLog))
	maps.Copy(copyMap, vs.committedLog)
	return copyMap
}

func (vs *volatileStore) State() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.state
}

func (vs *volatileStore) Prepare(txID uuid.UUID, newState int) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.committedLog[txID] {
		return errors.New("transaction already committed")
	}

	if vs.locked {
		if vs.lockedByTx == txID {
			return nil
		}
		return errors.New("node is locked by another transaction")
	}

	vs.locked = true
	vs.lockedByTx = txID
	vs.proposedValue = newState

	return nil
}

func (vs *volatileStore) Commit(txID uuid.UUID) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.committedLog[txID] {
		return nil
	}

	if !vs.locked || vs.lockedByTx != txID {
		return errors.New("invalid transaction commit")
	}

	vs.state = vs.proposedValue
	vs.locked = false
	vs.lockedByTx = uuid.Nil
	vs.committedLog[txID] = true

	return nil
}

func (vs *volatileStore) Abort(txID uuid.UUID) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.committedLog[txID] {
		return errors.New("cannot abort a committed transaction")
	}

	if !vs.locked || vs.lockedByTx != txID {
		return nil
	}

	vs.locked = false
	vs.lockedByTx = uuid.Nil
	vs.proposedValue = vs.state

	return nil
}

func NewVolatileStore(state int) VolatileStore {
	return &volatileStore{
		state:        state,
		committedLog: make(map[uuid.UUID]bool),
	}
}
