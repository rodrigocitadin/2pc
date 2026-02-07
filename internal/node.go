package internal

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strconv"
)

type Node interface {
	Transaction(value int) error
	State() int
	Talk(args TalkArgs, reply *string) error
}

type node struct {
	id      int
	address string
	peers   []Peer
	state   int
}

type TalkArgs struct {
	ID      int
	Message string
}

func (n *node) Talk(args TalkArgs, reply *string) error {
	if args.ID == n.id {
		log.Printf("[node:%d]: sending (%v) to all nodes", args.ID, args.Message)
		result := Broadcast[string](n.peers, "Node.Talk", args)
		for _, r := range result {
			log.Printf("[node:%d]: received (%s) back from node:%d", n.id, r.Value, r.PeerID)
		}
	} else {
		messages := [4]string{"love", "angry", "suspicion", "fear"}

		log.Printf("[node:%d]: received (%s) from node:%d", n.id, args.Message, args.ID)
		*reply = fmt.Sprintf("replying (%s) with %s", args.Message, messages[n.id])
	}

	return nil
}

func (n *node) State() int {
	return n.state
}

func (n *node) Transaction(value int) error {
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

	n := &node{
		id:      id,
		address: address,
		peers:   peers,
	}

	server := rpc.NewServer()
	server.RegisterName("Node", n)

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
