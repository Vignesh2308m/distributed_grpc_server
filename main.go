// server/main.go
// Leader server:
//   1. Receives SumTransactions from the client.
//   2. Partitions transactions evenly across registered worker nodes.
//   3. Fans-out parallel gRPC calls to every node (NodeService.SumPartition).
//   4. Aggregates the subtotals and returns the grand total to the client.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	pb "github.com/example/grpc-distributed-ledger/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// ─────────────────────────────────────────────────────────────────────────────
// nodeConn wraps a live connection to a single worker node
// ─────────────────────────────────────────────────────────────────────────────

type nodeConn struct {
	id     string
	client pb.NodeServiceClient
}

// ─────────────────────────────────────────────────────────────────────────────
// leaderServer implements pb.LeaderServiceServer
// ─────────────────────────────────────────────────────────────────────────────

type leaderServer struct {
	pb.UnimplementedLeaderServiceServer
	nodes []nodeConn
}

// SumTransactions is the single RPC exposed to external clients.
func (l *leaderServer) SumTransactions(ctx context.Context, req *pb.SumRequest) (*pb.SumResponse, error) {
	txs := req.GetTransactions()
	log.Printf("[leader] received %d transactions from client", len(txs))

	if len(l.nodes) == 0 {
		return nil, fmt.Errorf("leader has no registered worker nodes")
	}

	// ── Partition transactions round-robin across nodes ────────────────────
	partitions := make([][]*pb.Transaction, len(l.nodes))
	for i, tx := range txs {
		partitions[i%len(l.nodes)] = append(partitions[i%len(l.nodes)], tx)
	}

	for i, node := range l.nodes {
		log.Printf("[leader] → node %-10s gets %d transactions", node.id, len(partitions[i]))
	}

	// ── Fan-out: call every node in parallel ──────────────────────────────
	type result struct {
		resp *pb.NodeSumResponse
		err  error
	}

	results := make([]result, len(l.nodes))
	var wg sync.WaitGroup

	for i, node := range l.nodes {
		wg.Add(1)
		go func(idx int, n nodeConn, partition []*pb.Transaction) {
			defer wg.Done()

			nodeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			resp, err := n.client.SumPartition(nodeCtx, &pb.NodeSumRequest{
				NodeId:       n.id,
				Transactions: partition,
			})
			results[idx] = result{resp: resp, err: err}
		}(i, node, partitions[i])
	}

	wg.Wait()

	// ── Aggregate ──────────────────────────────────────────────────────────
	var grandTotal float64
	var nodeResults []*pb.NodeResult

	for i, r := range results {
		if r.err != nil {
			log.Printf("[leader] ✗ node %s returned error: %v", l.nodes[i].id, r.err)
			// Treat failed node as contributing 0 so the sum is still useful.
			nodeResults = append(nodeResults, &pb.NodeResult{
				NodeId:   l.nodes[i].id,
				Subtotal: 0,
				TxCount:  int32(len(partitions[i])),
			})
			continue
		}
		log.Printf("[leader] ✓ node %-10s subtotal=%.2f  txCount=%d",
			r.resp.GetNodeId(), r.resp.GetSubtotal(), r.resp.GetTxCount())

		grandTotal += r.resp.GetSubtotal()
		nodeResults = append(nodeResults, &pb.NodeResult{
			NodeId:   r.resp.GetNodeId(),
			Subtotal: r.resp.GetSubtotal(),
			TxCount:  r.resp.GetTxCount(),
		})
	}

	log.Printf("[leader] grand total = %.2f", grandTotal)

	return &pb.SumResponse{
		Total:       grandTotal,
		NumNodes:    int32(len(l.nodes)),
		NodeResults: nodeResults,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	port := flag.Int("port", 9000, "Port the leader listens on (for clients)")
	nodes := flag.String("nodes", "node-1=:50051,node-2=:50052,node-3=:50053",
		"Comma-separated list of id=addr pairs for worker nodes")
	flag.Parse()

	// ── Dial every worker node ─────────────────────────────────────────────
	var nodeConns []nodeConn
	for _, entry := range strings.Split(*nodes, ",") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			log.Fatalf("invalid node entry %q — expected id=addr", entry)
		}
		id, addr := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatalf("leader: failed to connect to node %s at %s: %v", id, addr, err)
		}
		nodeConns = append(nodeConns, nodeConn{id: id, client: pb.NewNodeServiceClient(conn)})
		log.Printf("[leader] registered node %s @ %s", id, addr)
	}

	// ── Start gRPC server ──────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("leader: failed to listen on %s: %v", addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)
	pb.RegisterLeaderServiceServer(grpcServer, &leaderServer{nodes: nodeConns})
	reflection.Register(grpcServer)

	log.Printf("🟡 leader listening on %s with %d nodes", addr, len(nodeConns))
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("leader: serve error: %v", err)
	}
}

// loggingInterceptor logs every RPC received by the leader.
func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	log.Printf("[leader] → RPC %s started", info.FullMethod)
	resp, err := handler(ctx, req)
	log.Printf("[leader] ← RPC %s finished in %s (err=%v)", info.FullMethod, time.Since(start), err)
	return resp, err
}
