package internal

import "github.com/google/uuid"

type NodeRPC interface {
	Prepare(args RequestArgs, reply *bool) error
	Commit(args RequestArgs, reply *bool) error
}

type nodeRPC struct {
	parent Node
}

type RequestArgs struct {
	TransactionID uuid.UUID
	Value         int
	SenderID      int
}

func (h *nodeRPC) Prepare(args RequestArgs, reply *bool) error {
	err := h.parent.prepare(args.TransactionID, args.Value, args.SenderID)

	if err != nil {
		*reply = false
	} else {
		*reply = true
	}

	return err
}

func (h *nodeRPC) Commit(args RequestArgs, reply *bool) error {
	err := h.parent.commit(args.TransactionID, args.Value, args.SenderID)

	if err != nil {
		*reply = false
	} else {
		*reply = true
	}

	return err
}

func newNodeRPC(n Node) NodeRPC {
	return &nodeRPC{parent: n}
}
