package internal

import (
	"errors"
	"net"
	"net/rpc"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rodrigocitadin/two-phase-commit/internal/store"
)

type Node interface {
	Transaction(value int) error
	State() int

	prepare(txID uuid.UUID, value, senderID int) error
	commit(txID uuid.UUID, value, senderID int) error
	abort(txID uuid.UUID, senderID int) error
	checkResult(result []Result[bool]) bool
	recover() error
	getStatus(txID uuid.UUID) (store.TransactionState, error)
}

type node struct {
	id            int
	address       string
	peers         []Peer
	stableStore   store.StableStore
	volatileStore store.VolatileStore
}

func (n *node) getStatus(txID uuid.UUID) (store.TransactionState, error) {
	return n.stableStore.GetTransactionState(txID)
}

func (n *node) recover() error {
	snapshotState, err := n.stableStore.LoadSnapshot()
	if err != nil {
		return err
	}
	n.volatileStore.Recover(snapshotState)

	lastTx, err := n.stableStore.RecoverLastState()
	if err != nil {
		return nil
	}

	switch lastTx.State {
	case store.TRANSACTION_COMMITTED:
		n.volatileStore.Recover(lastTx.Value)

	case store.TRANSACTION_PREPARED:
		n.volatileStore.Prepare(lastTx.TxID, lastTx.Value)
		go n.resolveAnomaly(lastTx.TxID, lastTx.SenderID, lastTx.Value)
	}

	currentState := n.volatileStore.State()
	if err := n.stableStore.SaveSnapshot(currentState); err != nil {
		return err
	}

	return n.stableStore.Truncate()
}

func (n *node) resolveAnomaly(txID uuid.UUID, senderID int, value int) {
	if senderID == n.id {
		n.abort(txID, senderID)
		return
	}

	var coordinator Peer
	for _, p := range n.peers {
		if p.ID() == senderID {
			coordinator = p
			break
		}
	}

	if coordinator == nil {
		n.abort(txID, senderID)
		return
	}

	for {
		var status store.TransactionState
		err := coordinator.Call("Node.GetStatus", txID, &status)

		if err == nil {
			if status == store.TRANSACTION_COMMITTED {
				n.commit(txID, value, senderID)
			} else {
				n.abort(txID, senderID)
			}
			return
		}

		// Backoff and retry if coordinator is not up yet
		time.Sleep(2 * time.Second)
	}
}

func (n *node) abort(txID uuid.UUID, senderID int) error {
	if err := n.stableStore.WriteAborted(txID, senderID); err != nil {
		return err
	}

	if err := n.volatileStore.Abort(txID); err != nil {
		return err
	}

	return nil
}

func (n *node) prepare(txID uuid.UUID, value, senderID int) error {
	if err := n.volatileStore.Prepare(txID, value); err != nil {
		return err
	}

	if err := n.stableStore.WritePrepared(txID, value, senderID); err != nil {
		if err := n.abort(txID, senderID); err != nil {
			return err
		}
		return err
	}

	return nil
}

func (n *node) commit(txID uuid.UUID, value, senderID int) error {
	if err := n.stableStore.WriteCommited(txID, value, senderID); err != nil {
		n.abort(txID, senderID)
		return err
	}

	if err := n.volatileStore.Commit(txID); err != nil {
		return err
	}

	return nil
}

func (n *node) State() int {
	return n.volatileStore.State()
}

func (n *node) checkResult(result []Result[bool]) bool {
	for _, v := range result {
		if v.Err != nil || v.Value == false {
			return false
		}
	}
	return true
}

func (n *node) Transaction(value int) error {
	txID, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	computedValue := n.volatileStore.State() + value

	// --- PHASE 1: PREPARE ---
	if err := n.prepare(txID, computedValue, n.id); err != nil {
		n.abort(txID, n.id)
		return errors.New("coordinator is busy/locked")
	}

	transactionArgs := RequestArgs{
		TxID:     txID,
		Value:    computedValue,
		SenderID: n.id,
	}

	prepareResults := Broadcast[bool](n.peers, "Node.Prepare", transactionArgs)
	if !n.checkResult(prepareResults) {
		Broadcast[bool](n.peers, "Node.Abort", RequestArgs{TxID: txID})
		n.abort(txID, n.id)
		return errors.New("consensus failed: a peer rejected or failed")
	}

	// --- PHASE 2: COMMIT ---
	if err := n.commit(txID, computedValue, n.id); err != nil {
		return err // rare critical failure and unsolved in this project/protocol
	}

	Broadcast[bool](n.peers, "Node.Commit", transactionArgs)
	return nil
}

func NewNode(id int, nodes map[int]string) (Node, error) {
	port := 3000 + id
	address := "localhost:" + strconv.Itoa(port)

	peers := make([]Peer, 0, len(nodes))
	for peerId, peerAddress := range nodes {
		if id != peerId && address != peerAddress {
			peer := NewPeer(peerId, peerAddress)
			peers = append(peers, peer)
		}
	}

	stableStore, err := store.NewStableStore(id)
	if err != nil {
		return nil, err
	}

	volatileStore := store.NewVolatileStore(0)

	n := &node{
		id:            id,
		address:       address,
		peers:         peers,
		stableStore:   stableStore,
		volatileStore: volatileStore,
	}

	if err := n.recover(); err != nil {
		return nil, err
	}

	nodeRPC := newNodeRPC(n)

	server := rpc.NewServer()
	server.RegisterName("Node", nodeRPC)

	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			conn, _ := l.Accept()
			go server.ServeConn(conn)
		}
	}()

	return n, nil
}
