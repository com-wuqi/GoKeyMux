package main

import (
	pb "GoKeyMux/proto"
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz"

// defaultNamedKeys are key names (IsRune=false) exercised by the named-key test.
var defaultNamedKeys = []string{"space", "enter", "tab", "escape", "backspace", "up", "down", "left", "right"}

func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	interval := flag.Duration("interval", 5*time.Second, "interval between a-z cycles")
	cycles := flag.Int("cycles", 5, "number of a-z key cycles to run")
	keys := flag.String("keys", strings.Join(defaultNamedKeys, ","), "comma-separated key names to test (IsRune=false)")
	keyDelay := flag.Duration("keyDelay", 300*time.Millisecond, "delay between named-key events")
	mouseDelay := flag.Duration("mouseDelay", 300*time.Millisecond, "delay between mouse events")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewRpcKeyServiceClient(conn)

	log.Println("=== [1/3] key cycle test (a-z) ===")
	runKeyCycles(ctx, client, *cycles, *interval)
	log.Println("=== [2/3] named key test ===")
	runNamedKeyTest(ctx, client, splitKeys(*keys), *keyDelay)
	log.Println("=== [3/3] mouse test ===")
	runMouseTest(ctx, client, *mouseDelay)
	log.Println("=== all tests done ===")
}

// runKeyCycles presses and releases a-z for the given number of cycles,
// waiting interval between cycles. It returns early if ctx is canceled.
func runKeyCycles(ctx context.Context, client pb.RpcKeyServiceClient, cycles int, interval time.Duration) {
	stream, err := client.KeyService(ctx)
	if err != nil {
		log.Fatalf("open key stream: %v", err)
	}

	for i := range cycles {
		if ctx.Err() != nil {
			break
		}

		start := time.Now()
		for _, r := range alphabet {
			key := string(r)
			if err := stream.Send(&pb.KeyInput{Key: key, IsRune: true, IsPressed: true}); err != nil {
				log.Fatalf("send press %q: %v", key, err)
			}
			time.Sleep(100 * time.Millisecond)
			if err := stream.Send(&pb.KeyInput{Key: key, IsRune: true, IsPressed: false}); err != nil {
				log.Fatalf("send release %q: %v", key, err)
			}
		}
		log.Printf("cycle %d/%d: cycled a-z in %s", i+1, cycles, time.Since(start))

		if i < cycles-1 && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case <-time.After(interval):
			}
		}
	}

	if _, err := stream.CloseAndRecv(); err != nil {
		log.Printf("close key stream: %v", err)
	}
}

// splitKeys parses a comma-separated key-name list, trimming whitespace and
// dropping empty entries.
func splitKeys(s string) []string {
	parts := strings.Split(s, ",")
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			keys = append(keys, p)
		}
	}
	return keys
}

// runNamedKeyTest presses and releases each named key (IsRune=false) once,
// waiting delay between events. It returns early if ctx is canceled.
func runNamedKeyTest(ctx context.Context, client pb.RpcKeyServiceClient, keys []string, delay time.Duration) {
	if len(keys) == 0 {
		return
	}

	stream, err := client.KeyService(ctx)
	if err != nil {
		log.Fatalf("open key stream: %v", err)
	}

	start := time.Now()
	for _, key := range keys {
		if ctx.Err() != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		log.Printf("press %q", key)
		if err := stream.Send(&pb.KeyInput{Key: key, IsRune: false, IsPressed: true}); err != nil {
			log.Fatalf("send press %q: %v", key, err)
		}
		if delay > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(delay):
			}
		}
		time.Sleep(100 * time.Millisecond)
		log.Printf("release %q", key)
		if err := stream.Send(&pb.KeyInput{Key: key, IsRune: false, IsPressed: false}); err != nil {
			log.Fatalf("send release %q: %v", key, err)
		}
		if delay > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(delay):
			}
		}
	}
	log.Printf("tested %d named keys in %s", len(keys), time.Since(start))

	if _, err := stream.CloseAndRecv(); err != nil {
		log.Printf("close key stream: %v", err)
	}
}

// runMouseTest sends a short sequence of mouse events (relative move, clicks,
// wheel, absolute move) to exercise the MouseService stream. A delay is
// inserted between events so movements are visible instead of instantaneous.
func runMouseTest(ctx context.Context, client pb.RpcKeyServiceClient, delay time.Duration) {
	stream, err := client.MouseService(ctx)
	if err != nil {
		log.Fatalf("open mouse stream: %v", err)
	}

	events := []*pb.MouseEvent{
		{Type: pb.MouseEvent_MOVE, X: 100, Y: 0},     // 相对右移 100px
		{Type: pb.MouseEvent_MOVE, X: 0, Y: 100},     // 相对下移 100px
		{Type: pb.MouseEvent_MOVE, X: -100, Y: -100}, // 相对移回原位
		{Type: pb.MouseEvent_LEFT_DOWN},
		{Type: pb.MouseEvent_LEFT_UP},
		{Type: pb.MouseEvent_RIGHT_DOWN},
		{Type: pb.MouseEvent_RIGHT_UP},
		{Type: pb.MouseEvent_MIDDLE_DOWN},
		{Type: pb.MouseEvent_MIDDLE_UP},
		{Type: pb.MouseEvent_SCROLL, ScrollDelta: 3},     // 向上滚动
		{Type: pb.MouseEvent_SCROLL, ScrollDelta: -3},    // 向下滚动
		{Type: pb.MouseEvent_ABS_MOVE, X: 5000, Y: 5000}, // 移到屏幕中心
	}

	start := time.Now()
	for _, ev := range events {
		log.Printf("type=%s x=%d y=%d scroll=%d", ev.GetType(), ev.GetX(), ev.GetY(), ev.GetScrollDelta())
		if err := stream.Send(ev); err != nil {
			log.Fatalf("send mouse event %s: %v", ev.GetType(), err)
		}
		if delay > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(delay):
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("sent %d mouse events in %s", len(events), time.Since(start))

	ret, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("close mouse stream: %v", err)
	}
	log.Printf("mouse test done: isAllDone=%v metaData=%s", ret.GetIsAllDone(), ret.GetMetaData())
}
