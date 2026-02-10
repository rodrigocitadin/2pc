package internal

import "github.com/google/uuid"

type RequestArgs struct {
	TxID     uuid.UUID
	Value    int
	SenderID int
}

type NodeRPC interface {
	Abort(args RequestArgs, reply *bool) error
	Prepare(args RequestArgs, reply *bool) error
	Commit(args RequestArgs, reply *bool) error
}

type nodeRPC struct {
	parent Node
}

func (n *nodeRPC) Abort(args RequestArgs, reply *bool) error {
	err := n.parent.abort(args.TxID, args.SenderID)

	if err != nil {
		*reply = false
	} else {
		*reply = true
	}

	return err
}

func (n *nodeRPC) Prepare(args RequestArgs, reply *bool) error {
	err := n.parent.prepare(args.TxID, args.Value, args.SenderID)

	if err != nil {
		*reply = false
	} else {
		*reply = true
	}

	return err
}

func (n *nodeRPC) Commit(args RequestArgs, reply *bool) error {
	err := n.parent.commit(args.TxID, args.Value, args.SenderID)

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
