// node/main.go
// Worker node: listens for SumPartition requests from the leader,
// sums its assigned transactions, and returns the subtotal.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/example/grpc-distributed-ledger/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ─────────────────────────────────────────────────────────────────────────────
// nodeServer implements pb.NodeServiceServer
// ─────────────────────────────────────────────────────────────────────────────

type nodeServer struct {
	pb.UnimplementedNodeServiceServer
	id string
}

// SumPartition adds up all transaction amounts in the request and returns the
// subtotal together with the count of processed transactions.
func (n *nodeServer) SumPartition(_ context.Context, req *pb.NodeSumRequest) (*pb.NodeSumResponse, error) {
	log.Printf("[node %s] received %d transactions to process", n.id, len(req.GetTransactions()))

	var subtotal float64
	for _, tx := range req.GetTransactions() {
		log.Printf("[node %s]   tx %-10s  label=%-20s  amount=%.2f", n.id, tx.GetId(), tx.GetLabel(), tx.GetAmount())
		subtotal += tx.GetAmount()
	}

	log.Printf("[node %s] subtotal = %.2f", n.id, subtotal)

	return &pb.NodeSumResponse{
		NodeId:   n.id,
		Subtotal: subtotal,
		TxCount:  int32(len(req.GetTransactions())),
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// main
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	port := flag.Int("port", 50051, "Port this node listens on")
	id := flag.String("id", "node-1", "Unique identifier for this node")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("node %s: failed to listen on %s: %v", *id, addr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor(*id)),
	)
	pb.RegisterNodeServiceServer(grpcServer, &nodeServer{id: *id})

	// Register reflection so tools like grpcurl can inspect the service.
	reflection.Register(grpcServer)

	log.Printf("🟢 node %s listening on %s", *id, addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("node %s: serve error: %v", *id, err)
	}
}

// loggingInterceptor logs every incoming RPC at the node level.
func loggingInterceptor(nodeID string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		log.Printf("[node %s] → RPC %s called", nodeID, info.FullMethod)
		resp, err := handler(ctx, req)
		if err != nil {
			log.Printf("[node %s] ✗ RPC %s error: %v", nodeID, info.FullMethod, err)
		} else {
			log.Printf("[node %s] ✓ RPC %s completed", nodeID, info.FullMethod)
		}
		return resp, err
	}
}
