package main

import (
	pb "GoKeyMux/proto"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz"

func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	interval := flag.Duration("interval", 5*time.Second, "interval between a-z cycles")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewRpcKeyServiceClient(conn)

	stream, err := client.KeyService(ctx)
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			if _, err := stream.CloseAndRecv(); err != nil {
				log.Printf("close stream: %v", err)
			}
			return
		default:
		}

		start := time.Now()
		for _, r := range alphabet {
			key := string(r)
			if err := stream.Send(&pb.KeyInput{Key: key, IsRune: true, IsPressed: true}); err != nil {
				log.Fatalf("send press %q: %v", key, err)
			}
			if err := stream.Send(&pb.KeyInput{Key: key, IsRune: true, IsPressed: false}); err != nil {
				log.Fatalf("send release %q: %v", key, err)
			}
		}
		log.Printf("cycled a-z in %s, next cycle in %s", time.Since(start), *interval)

		select {
		case <-ctx.Done():
		case <-time.After(*interval):
		}
	}
}
