package main

import (
	pb "GoKeyMux/proto"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	mode := flag.String("mode", "stream", "stream|unary|both")
	concurrency := flag.Int("c", 10, "number of concurrent workers")
	msgs := flag.Int("n", 100000, "messages/requests per worker (ignored when -d > 0)")
	dur := flag.Duration("d", 0, "duration per worker; overrides -n when > 0")
	key := flag.String("key", "space", "key name or rune string")
	isRune := flag.Bool("isRune", false, "treat -key as a rune string")
	isPressed := flag.Bool("isPressed", true, "press (true) or release (false)")
	warmup := flag.Duration("warmup", 0, "warmup duration before measuring")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile := flag.String("memprofile", "", "write heap profile to file")
	pprofAddr := flag.String("pprof", "", "serve pprof on host:port (client)")
	flag.Parse()

	if *pprofAddr != "" {
		go func() { log.Println(http.ListenAndServe(*pprofAddr, nil)) }()
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewRpcKeyServiceClient(conn)

	switch *mode {
	case "stream":
		runStream(client, *concurrency, *msgs, *dur, *key, *isRune, *isPressed, *warmup)
	case "unary":
		runUnary(client, *concurrency, *msgs, *dur, *key, *isRune, *isPressed, *warmup)
	case "both":
		runStream(client, *concurrency, *msgs, *dur, *key, *isRune, *isPressed, *warmup)
		runUnary(client, *concurrency, *msgs, *dur, *key, *isRune, *isPressed, *warmup)
	default:
		log.Fatalf("unknown mode %q", *mode)
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal(err)
		}
	}
}

func streamOnce(client pb.RpcKeyServiceClient, c, n int, dur time.Duration, key string, isRune, isPressed bool) (total, failed uint64, elapsed time.Duration) {
	ctx := context.Background()
	start := time.Now()

	var wg sync.WaitGroup
	for range c {
		wg.Go(func() {
			stream, err := client.KeyService(ctx)
			if err != nil {
				atomic.AddUint64(&failed, 1)
				return
			}
			sent := 0
			for {
				if dur > 0 && time.Since(start) >= dur {
					break
				}
				if dur == 0 && sent >= n {
					break
				}
				err := stream.Send(&pb.KeyInput{Key: key, IsRune: isRune, IsPressed: isPressed})
				if err == io.EOF {
					break
				}
				if err != nil {
					atomic.AddUint64(&failed, 1)
					break
				}
				sent++
				atomic.AddUint64(&total, 1)
			}
			if _, err := stream.CloseAndRecv(); err != nil {
				atomic.AddUint64(&failed, 1)
			}
		})
	}
	wg.Wait()
	return atomic.LoadUint64(&total), atomic.LoadUint64(&failed), time.Since(start)
}

func runStream(client pb.RpcKeyServiceClient, c, n int, dur time.Duration, key string, isRune, isPressed bool, warmup time.Duration) {
	if warmup > 0 {
		streamOnce(client, c, 0, warmup, key, isRune, isPressed)
	}
	total, failed, elapsed := streamOnce(client, c, n, dur, key, isRune, isPressed)
	tps := 0.0
	if elapsed.Seconds() > 0 {
		tps = float64(total) / elapsed.Seconds()
	}
	fmt.Printf("stream: total=%d elapsed=%s tps=%.0f failed=%d\n", total, elapsed, tps, failed)
}

func unaryOnce(client pb.RpcKeyServiceClient, c, n int, dur time.Duration, key string, isRune, isPressed bool) (latencies []time.Duration, failed uint64, elapsed time.Duration) {
	ctx := context.Background()
	start := time.Now()
	perWorker := make([][]time.Duration, c)

	var wg sync.WaitGroup
	for i := range c {
		wg.Add(1)
		go func(wi int) {
			defer wg.Done()
			sent := 0
			for {
				if dur > 0 {
					if time.Since(start) >= dur {
						return
					}
				} else if sent >= n {
					return
				}
				t0 := time.Now()
				_, err := client.KeyServiceDebug(ctx, &pb.KeyInputDebug{Key: key, IsRune: isRune, IsPressed: isPressed})
				if err != nil {
					atomic.AddUint64(&failed, 1)
					continue
				}
				perWorker[wi] = append(perWorker[wi], time.Since(t0))
				sent++
			}
		}(i)
	}
	wg.Wait()

	var all []time.Duration
	for _, wl := range perWorker {
		all = append(all, wl...)
	}
	return all, atomic.LoadUint64(&failed), time.Since(start)
}

func runUnary(client pb.RpcKeyServiceClient, c, n int, dur time.Duration, key string, isRune, isPressed bool, warmup time.Duration) {
	if warmup > 0 {
		unaryOnce(client, c, 0, warmup, key, isRune, isPressed)
	}
	latencies, failed, elapsed := unaryOnce(client, c, n, dur, key, isRune, isPressed)
	nr := len(latencies)
	if nr == 0 {
		fmt.Printf("unary: no successful requests (failed=%d)\n", failed)
		return
	}

	slices.Sort(latencies)
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	avg := sum / time.Duration(nr)
	p := func(q float64) time.Duration {
		idx := int(q * float64(nr))
		if idx >= nr {
			idx = nr - 1
		}
		return latencies[idx]
	}
	rps := 0.0
	if elapsed.Seconds() > 0 {
		rps = float64(nr) / elapsed.Seconds()
	}
	fmt.Printf("unary: total=%d elapsed=%s rps=%.0f avg=%s p50=%s p90=%s p95=%s p99=%s p999=%s max=%s failed=%d\n",
		nr, elapsed, rps, avg, p(0.50), p(0.90), p(0.95), p(0.99), p(0.999), latencies[nr-1], failed)
}
