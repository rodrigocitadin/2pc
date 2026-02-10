package internal

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

// cleanLogs removes the log directory to ensure a fresh start
func cleanLogs() {
	_ = os.RemoveAll("./logs")
}

// generateNodes creates a cluster configuration for a specific test offset
// We use offsets (10, 20, 30) to avoid port conflicts between tests
func generateNodes(startID, count int) map[int]string {
	nodes := make(map[int]string)
	for i := 0; i < count; i++ {
		id := startID + i
		nodes[id] = "localhost:" + strconv.Itoa(3000+id)
	}
	return nodes
}

// createCluster spins up the actual node instances
func createCluster(t *testing.T, nodesConfig map[int]string) []Node {
	var nodes []Node
	for id := range nodesConfig {
		n, err := NewNode(id, nodesConfig)
		if err != nil {
			// Cleanup already created nodes on failure
			for _, created := range nodes {
				created.Close()
			}
			t.Fatalf("Failed to create node %d: %v", id, err)
		}
		nodes = append(nodes, n)
	}
	// Give time for listeners to start
	time.Sleep(100 * time.Millisecond)
	return nodes
}

// teardown closes all nodes and removes logs
func teardown(nodes []Node) {
	for _, n := range nodes {
		n.Close()
	}
	cleanLogs()
}

func TestTwoPhaseCommit_HappyPath(t *testing.T) {
	cleanLogs()
	// Use IDs 10, 11, 12
	nodesConfig := generateNodes(10, 3)
	nodes := createCluster(t, nodesConfig)
	defer teardown(nodes)

	coordinator := nodes[0]

	// Execute Transaction: Add 10 to current state (0)
	t.Log("Coordinator initiating transaction: +10")
	err := coordinator.Transaction(10)
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Wait for propagation
	time.Sleep(200 * time.Millisecond)

	// Verify all nodes have committed
	expectedState := 10
	for _, n := range nodes {
		// We need to type assert to access ID for logging
		nodeImpl := n.(*node)
		if n.State() != expectedState {
			t.Errorf("Node %d state mismatch. Want %d, Got %d", nodeImpl.id, expectedState, n.State())
		}
	}
}

func TestTwoPhaseCommit_AbortOnPrepareFailure(t *testing.T) {
	cleanLogs()
	// Use IDs 20, 21
	nodesConfig := generateNodes(20, 2)
	nodes := createCluster(t, nodesConfig)
	defer teardown(nodes)

	coordinator := nodes[0]
	participant := nodes[1]

	// SIMULATION: Manually lock the participant to simulate a busy/locked resource
	// This forces the Prepare phase to fail on the participant.
	partImpl := participant.(*node)
	fakeTxID := uuid.New()
	partImpl.volatileStore.Prepare(fakeTxID, 999)

	t.Log("Participant manually locked. Initiating transaction...")
	err := coordinator.Transaction(50)

	if err == nil {
		t.Fatal("Expected transaction to fail, but it succeeded")
	} else {
		t.Logf("Transaction failed as expected: %v", err)
	}

	// Verify State remains 0 on Coordinator
	if coordinator.State() != 0 {
		t.Errorf("Coordinator state altered on abort! Want 0, Got %d", coordinator.State())
	}

	// Verify Participant is still locked by the fake transaction (or at least not updated)
	if participant.State() != 0 {
		t.Errorf("Participant state altered! Want 0, Got %d", participant.State())
	}
}

func TestNodeRecovery_Persistence(t *testing.T) {
	cleanLogs()
	// Use IDs 30, 31
	nodesConfig := generateNodes(30, 2)

	// 1. Start Cluster and Commit Data
	nodes := createCluster(t, nodesConfig)
	coordinator := nodes[0]

	err := coordinator.Transaction(100)
	if err != nil {
		teardown(nodes)
		t.Fatalf("Setup transaction failed: %v", err)
	}

	// Ensure data is written
	time.Sleep(100 * time.Millisecond)

	// 2. Kill a participant (Simulate crash by closing it)
	// We specifically want to restart Node 31
	victimID := 31
	var victimNode Node
	for _, n := range nodes {
		if n.(*node).id == victimID {
			victimNode = n
			break
		}
	}

	t.Logf("Crashing node %d...", victimID)
	victimNode.Close()

	// 3. Restart the node
	// We create a new instance with the same ID. It should read from disk.
	t.Logf("Recovering node %d...", victimID)
	recoveredNode, err := NewNode(victimID, nodesConfig)
	if err != nil {
		t.Fatalf("Failed to restart node: %v", err)
	}
	defer recoveredNode.Close() // Ensure this gets closed too

	// 4. Verify State Recovery
	if recoveredNode.State() != 100 {
		t.Errorf("Recovery failed. Expected State 100, Got %d", recoveredNode.State())
	} else {
		t.Log("Node successfully recovered state 100 from WAL.")
	}

	// Teardown the rest
	// Note: victimNode is already closed, so we close the others.
	for _, n := range nodes {
		if n.(*node).id != victimID {
			n.Close()
		}
	}
}

func TestCoordinator_Timeout_HandlesPeerFailure(t *testing.T) {
	cleanLogs()
	// Use IDs 40, 41
	nodesConfig := generateNodes(40, 2)
	nodes := createCluster(t, nodesConfig)
	defer teardown(nodes)

	coordinator := nodes[0]
	participant := nodes[1]

	// Kill the participant immediately
	t.Log("Killing participant to simulate network partition/crash...")
	participant.Close()

	// Attempt transaction
	t.Log("Initiating transaction expecting timeout/connection failure...")
	err := coordinator.Transaction(10)

	if err == nil {
		t.Fatal("Transaction succeeded despite dead peer!")
	}

	t.Logf("Transaction failed as expected: %v", err)

	if coordinator.State() != 0 {
		t.Errorf("Coordinator state inconsistent. Want 0, Got %d", coordinator.State())
	}
}

func TestSequentialTransactions(t *testing.T) {
	cleanLogs()
	// Use IDs 50, 51, 52
	nodesConfig := generateNodes(50, 3)
	nodes := createCluster(t, nodesConfig)
	defer teardown(nodes)

	coordinator := nodes[0]

	// Run 5 sequential transactions
	for i := 1; i <= 5; i++ {
		err := coordinator.Transaction(1) // +1 each time
		if err != nil {
			t.Fatalf("Transaction %d failed: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// Expect total = 5
	for _, n := range nodes {
		if n.State() != 5 {
			t.Errorf("Node %d state mismatch. Want 5, Got %d", n.(*node).id, n.State())
		}
	}
}
