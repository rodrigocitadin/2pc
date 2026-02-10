package internal

import (
	"errors"
	"net"
	"net/rpc"
	"strconv"

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
}

type node struct {
	id            int
	address       string
	peers         []Peer
	stableStore   store.StableStore
	volatileStore store.VolatileStore
}

func (n *node) recover() error {
	lastTx, err := n.stableStore.RecoverLastState()
	if err != nil {

	}

	if lastTx.State == store.TRANSACTION_PREPARED {
		if err := n.commit(lastTx.TxID, lastTx.Value, lastTx.SenderID); err != nil {
			return err
		}
	} else {
		n.volatileStore.Recover(lastTx.Value)
	}

	n.stableStore.Truncate() // error here is not a big problem
	return nil
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
	if err := n.volatileStore.Commit(txID); err != nil {
		if err := n.abort(txID, senderID); err != nil {
			return err
		}
		return err
	}

	if err := n.stableStore.WriteCommited(txID, value, senderID); err != nil {
		if err := n.abort(txID, senderID); err != nil {
			return err
		}
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
