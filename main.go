package main

import (
	"strconv"

	"github.com/rodrigocitadin/two-phase-commit/internal"
)

const (
	numberOfNodes = 4
)

func main() {
	nodeAddresses := make(map[int]string, numberOfNodes)
	for i := range numberOfNodes {
		port := 3000 + i
		nodeAddresses[i] = "localhost:" + strconv.Itoa(port)
	}

	nodes := make([]internal.Node, 0, numberOfNodes)
	for i := range numberOfNodes {
		node, err := internal.NewNode(i, nodeAddresses)
		if err != nil {
			panic(err)
		}

		nodes = append(nodes, node)
	}

	reply := new(string)
	nodes[0].Talk(
		internal.TalkArgs{
			ID:      0,
			Message: "testing nodes",
		},
		reply,
	)
}
