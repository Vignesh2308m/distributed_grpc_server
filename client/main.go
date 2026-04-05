// client/main.go
// Client: connects to the leader, sends a batch of sample transactions,
// and pretty-prints the aggregated result.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/example/grpc-distributed-ledger/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// makeSampleTransactions produces n random transactions so we have something
// interesting to distribute across the nodes.
func makeSampleTransactions(n int) []*pb.Transaction {
	labels := []string{
		"groceries", "rent", "salary", "refund", "subscription",
		"transfer", "bonus", "utilities", "insurance", "dividend",
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	txs := make([]*pb.Transaction, n)
	for i := range txs {
		txs[i] = &pb.Transaction{
			Id:     fmt.Sprintf("tx-%04d", i+1),
			Amount: float64(rng.Intn(9901)+100) / 100.0, // $1.00 – $100.00
			Label:  labels[rng.Intn(len(labels))],
		}
	}
	return txs
}

// ─────────────────────────────────────────────────────────────────────────────
// main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	leaderAddr := flag.String("leader", "localhost:9000", "Leader gRPC address")
	numTx := flag.Int("txs", 12, "Number of sample transactions to send")
	flag.Parse()

	// ── Connect to leader ──────────────────────────────────────────────────
	conn, err := grpc.NewClient(*leaderAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("client: failed to connect to leader at %s: %v", *leaderAddr, err)
	}
	defer conn.Close()

	client := pb.NewLeaderServiceClient(conn)

	// ── Build request ──────────────────────────────────────────────────────
	txs := makeSampleTransactions(*numTx)

	fmt.Printf("\n╔══════════════════════════════════════════════════════╗\n")
	fmt.Printf("║        gRPC Distributed Transaction Ledger           ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════╝\n\n")

	fmt.Printf("📤 Sending %d transactions to leader @ %s\n\n", len(txs), *leaderAddr)
	fmt.Printf("  %-12s  %-22s  %10s\n", "ID", "Label", "Amount")
	fmt.Printf("  %s\n", "─────────────────────────────────────────────────")

	var clientTotal float64
	for _, tx := range txs {
		fmt.Printf("  %-12s  %-22s  %10.2f\n", tx.GetId(), tx.GetLabel(), tx.GetAmount())
		clientTotal += tx.GetAmount()
	}

	fmt.Printf("  %s\n", "─────────────────────────────────────────────────")
	fmt.Printf("  %-12s  %-22s  %10.2f\n\n", "", "Client-side total:", clientTotal)

	// ── Call leader ────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := client.SumTransactions(ctx, &pb.SumRequest{Transactions: txs})
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("client: SumTransactions failed: %v", err)
	}

	// ── Pretty-print response ──────────────────────────────────────────────
	fmt.Printf("📥 Response from leader (%.1fms round-trip, %d nodes)\n\n",
		float64(elapsed.Microseconds())/1000.0, resp.GetNumNodes())

	fmt.Printf("  %-12s  %10s  %10s\n", "Node", "Subtotal", "Tx Count")
	fmt.Printf("  %s\n", "──────────────────────────────────────────")

	for _, nr := range resp.GetNodeResults() {
		fmt.Printf("  %-12s  %10.2f  %10d\n",
			nr.GetNodeId(), nr.GetSubtotal(), nr.GetTxCount())
	}

	fmt.Printf("  %s\n", "──────────────────────────────────────────")
	fmt.Printf("  %-12s  %10.2f\n\n", "GRAND TOTAL", resp.GetTotal())

	// Sanity check — both sides should agree (floating-point rounding aside)
	diff := clientTotal - resp.GetTotal()
	if diff < -0.01 || diff > 0.01 {
		fmt.Printf("⚠️  WARNING: client total (%.2f) differs from leader total (%.2f)!\n",
			clientTotal, resp.GetTotal())
	} else {
		fmt.Printf("✅ Grand total verified: %.2f\n\n", resp.GetTotal())
	}
}
