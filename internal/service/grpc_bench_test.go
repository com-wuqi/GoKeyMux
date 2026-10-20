package service

import (
	"GoKeyMux/internal/engine"
	"context"
	"fmt"
	"net"
	"testing"

	pb "GoKeyMux/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// These benchmarks run a real gRPC server over an in-memory bufconn listener,
// so they include protobuf framing, the gRPC runtime and the logging
// interceptors. They intentionally run client and server in the same process:
// use cmd/loadtest (bench/e2e.ps1) for loopback TCP numbers.

func newBenchGRPCClient(b *testing.B, e *engine.Engine) pb.RpcKeyServiceClient {
	b.Helper()

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(
		grpc.UnaryInterceptor(UnaryServerLogging),
		grpc.StreamInterceptor(StreamServerLogging),
	)
	pb.RegisterRpcKeyServiceServer(server, NewGoKeyMuxService(e))
	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		server.Stop()
		_ = listener.Close()
		b.Fatalf("grpc.NewClient: %v", err)
	}

	b.Cleanup(func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	})
	return pb.NewRpcKeyServiceClient(conn)
}

// BenchmarkGRPCDispatch measures the service's dispatch method without any
// transport or protobuf work: key resolution plus engine submission.
func BenchmarkGRPCDispatch(b *testing.B) {
	e := newGRPCBenchEngine(b)
	service := NewGoKeyMuxService(e)
	ctx := context.Background()

	cases := []struct {
		name    string
		key     string
		isRune  bool
		keysPer int
	}{
		{"name", "space", false, 1},
		{"rune", "w", true, 1},
		{"chord4", "wasd", true, 4},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Alternate press/release so the held-key set stays bounded.
				if err := service.dispatch(ctx, tc.key, tc.isRune, i%2 == 0); err != nil {
					b.Fatalf("dispatch: %v", err)
				}
			}
			reportThroughput(b, tc.keysPer)
		})
	}
}

// BenchmarkGRPCUnaryKeyServiceDebug is the full unary path: request
// marshalling, interceptors, dispatch, response marshalling.
func BenchmarkGRPCUnaryKeyServiceDebug(b *testing.B) {
	e := newGRPCBenchEngine(b)
	client := newBenchGRPCClient(b, e)
	ctx := context.Background()
	req := &pb.KeyInputDebug{Key: "space", IsRune: false, IsPressed: true}

	// Establish the HTTP/2 connection outside the timed loop; otherwise the
	// first iteration pays for the dial.
	if _, err := client.KeyServiceDebug(ctx, req); err != nil {
		b.Fatalf("warmup KeyServiceDebug: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.KeyServiceDebug(ctx, req); err != nil {
			b.Fatalf("KeyServiceDebug: %v", err)
		}
	}
	reportThroughput(b, 1)
}

// BenchmarkGRPCUnaryParallel measures concurrent unary clients. The engine
// still serializes keyboard injection, so this mostly shows gRPC-side
// contention plus the submit queue.
func BenchmarkGRPCUnaryParallel(b *testing.B) {
	e := newGRPCBenchEngine(b)
	client := newBenchGRPCClient(b, e)
	ctx := context.Background()

	if _, err := client.KeyServiceDebug(ctx, &pb.KeyInputDebug{Key: "space", IsPressed: true}); err != nil {
		b.Fatalf("warmup KeyServiceDebug: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(iter *testing.PB) {
		for iter.Next() {
			if _, err := client.KeyServiceDebug(ctx, &pb.KeyInputDebug{Key: "space", IsPressed: true}); err != nil {
				b.Errorf("KeyServiceDebug: %v", err)
				return
			}
		}
	})
	reportThroughput(b, 1)
}

// BenchmarkGRPCStreamKeyService varies the number of messages per client
// stream, separating stream setup cost from per-message cost.
func BenchmarkGRPCStreamKeyService(b *testing.B) {
	e := newGRPCBenchEngine(b)
	client := newBenchGRPCClient(b, e)
	ctx := context.Background()

	// Establish the HTTP/2 connection before the timed sub-benchmarks.
	warmup, err := client.KeyService(ctx)
	if err != nil {
		b.Fatalf("warmup KeyService: %v", err)
	}
	if _, err := warmup.CloseAndRecv(); err != nil {
		b.Fatalf("warmup CloseAndRecv: %v", err)
	}

	for _, messages := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("msgs%d", messages), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				stream, err := client.KeyService(ctx)
				if err != nil {
					b.Fatalf("KeyService: %v", err)
				}
				for j := 0; j < messages; j++ {
					if err := stream.Send(&pb.KeyInput{Key: "space", IsPressed: (i+j)%2 == 0}); err != nil {
						b.Fatalf("send: %v", err)
					}
				}
				if _, err := stream.CloseAndRecv(); err != nil {
					b.Fatalf("CloseAndRecv: %v", err)
				}
			}
			reportThroughput(b, messages)
		})
	}
}

// BenchmarkGRPCMouseService measures the mouse streaming path end to end.
func BenchmarkGRPCMouseService(b *testing.B) {
	e := newGRPCBenchEngine(b)
	client := newBenchGRPCClient(b, e)
	ctx := context.Background()
	events := make([]*pb.MouseEvent, 32)
	for i := range events {
		events[i] = &pb.MouseEvent{Type: pb.MouseEvent_MOVE, X: int32(i + 1), Y: int32(-(i + 1))}
	}

	warmup, err := client.MouseService(ctx)
	if err != nil {
		b.Fatalf("warmup MouseService: %v", err)
	}
	if _, err := warmup.CloseAndRecv(); err != nil {
		b.Fatalf("warmup CloseAndRecv: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := client.MouseService(ctx)
		if err != nil {
			b.Fatalf("MouseService: %v", err)
		}
		for _, ev := range events {
			if err := stream.Send(ev); err != nil {
				b.Fatalf("send: %v", err)
			}
		}
		if _, err := stream.CloseAndRecv(); err != nil {
			b.Fatalf("CloseAndRecv: %v", err)
		}
	}
	reportThroughput(b, len(events))
}
