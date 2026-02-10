package main

import (
	"fmt"
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

	nodes[0].Transaction(1)
	logNodesState(nodes)

	nodes[0].Transaction(1)
	logNodesState(nodes)

	nodes[0].Transaction(1)
	logNodesState(nodes)
}

func logNodesState(nodes []internal.Node) {
	fmt.Printf("node0: %v\n", nodes[0].State())
	fmt.Printf("node1: %v\n", nodes[1].State())
	fmt.Printf("node2: %v\n", nodes[2].State())
	fmt.Printf("node3: %v\n", nodes[3].State())
}
