package main

import (
	pb "GoKeyMux/proto"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type GoKeyMuxService struct {
	pb.UnimplementedRpcKeyServiceServer
	engine *Engine
}

func NewGoKeyMuxService(engine *Engine) *GoKeyMuxService {
	return &GoKeyMuxService{engine: engine}
}

func (s *GoKeyMuxService) KeyService(stream pb.RpcKeyService_KeyServiceServer) error {
	var errCount int
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.KeyReturn{
				IsAllDone: errCount == 0,
				MetaData:  time.Now().String(),
			})
		}
		if err != nil {
			return err
		}
		if err := s.dispatch(stream.Context(), req.GetKey(), req.GetIsRune(), req.GetIsPressed()); err != nil {
			errCount++
			slog.Warn("key dispatch failed", "key", req.GetKey(), "isRune", req.GetIsRune(), "isPressed", req.GetIsPressed(), "err", err)
			continue
		}
		if slog.Default().Enabled(stream.Context(), slog.LevelDebug) {
			slog.Debug("Received key request", "key", req.GetKey(), "isPressed", req.GetIsPressed(), "isRune", req.GetIsRune())
		}
	}
}

func (s *GoKeyMuxService) KeyServiceDebug(ctx context.Context, req *pb.KeyInputDebug) (*pb.KeyReturnDebug, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if err := s.dispatch(ctx, req.GetKey(), req.GetIsRune(), req.GetIsPressed()); err != nil {
		return nil, status.Errorf(codes.Internal, "dispatch failed: %v", err)
	}
	return &pb.KeyReturnDebug{
		IsFinished: true,
		Key:        req.GetKey(),
		TimeStamp:  time.Now().Format(time.RFC3339Nano),
	}, nil
}

// dispatch resolves a key spec and presses or releases it on the engine. When
// isRune is true the key is parsed as a rune string (multiple runes form a
// simultaneous chord); otherwise it is looked up as a key name.
func (s *GoKeyMuxService) dispatch(ctx context.Context, key string, isRune bool, isPressed bool) error {
	if s.engine == nil {
		return errors.New("engine is not initialized")
	}
	var keys []KeyCodes
	if isRune {
		var ok bool
		keys, ok = KeyCodesFromRunes(key)
		if !ok {
			return fmt.Errorf("unknown rune sequence %q", key)
		}
	} else {
		kc, ok := KeyCodesFromName(key)
		if !ok {
			return fmt.Errorf("unknown key %q", key)
		}
		keys = []KeyCodes{kc}
	}
	if isPressed {
		return s.engine.EnginePress(ctx, keys...)
	}
	return s.engine.EngineRelease(ctx, keys...)
}

// logLevelForCode maps a gRPC status code to the slog level used for the
// request/stream log line. Successful calls are debug-level, so they only show
// when the configured log level is debug.
func logLevelForCode(code codes.Code) slog.Level {
	switch code {
	case codes.OK:
		return slog.LevelDebug
	case codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unknown:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

func UnaryServerLogging(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)
	st, _ := status.FromError(err)
	slog.Default().LogAttrs(ctx, logLevelForCode(st.Code()), "grpc.unary",
		slog.String("method", info.FullMethod),
		slog.Duration("duration", duration),
		slog.String("status", st.Code().String()),
	)
	return resp, err
}

// StreamServerLogging is the streaming counterpart of UnaryServerLogging. The
// main keyService RPC is a client-streaming method, so without this interceptor
// no log line is emitted for it (grpc.UnaryInterceptor only covers unary calls).
func StreamServerLogging(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	duration := time.Since(start)
	st, _ := status.FromError(err)
	slog.Default().LogAttrs(ss.Context(), logLevelForCode(st.Code()), "grpc.stream",
		slog.String("method", info.FullMethod),
		slog.Bool("is_client_stream", info.IsClientStream),
		slog.Bool("is_server_stream", info.IsServerStream),
		slog.Duration("duration", duration),
		slog.String("status", st.Code().String()),
	)
	return err
}

func StartService(engine *Engine) (*grpc.Server, <-chan error, error) {
	ln, err := net.Listen("tcp4", GlobalConfig.GRPCAddress)
	if err != nil {
		return nil, nil, err
	}
	hs := health.NewServer()
	hs.SetServingStatus("GoKeyMux.rpcKeyService", healthpb.HealthCheckResponse_SERVING)
	server := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:              time.Duration(GlobalConfig.GRPCKeepaliveTime) * time.Second,
			Timeout:           time.Duration(GlobalConfig.GRPCKeepaliveTimeOut) * time.Second,
			MaxConnectionIdle: time.Duration(GlobalConfig.GRPCKeepaliveMaxConnectionIdle) * time.Second,
		}),
		// 服务端 enforcement：限制客户端 ping 频率与无流 ping。
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Duration(GlobalConfig.GRPCEnforcementPolicyMinTime) * time.Second,
			PermitWithoutStream: GlobalConfig.GRPCEnforcementPermitWithoutStream,
		}),
		grpc.UnaryInterceptor(UnaryServerLogging),
		grpc.StreamInterceptor(StreamServerLogging),
	)
	pb.RegisterRpcKeyServiceServer(server, NewGoKeyMuxService(engine))
	healthpb.RegisterHealthServer(server, hs)
	serveErr := make(chan error, 1)
	go func() {
		defer close(serveErr)
		if err := server.Serve(ln); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			slog.Error("Server stopped", "err", err)
			serveErr <- err
		}
	}()
	return server, serveErr, nil
}
