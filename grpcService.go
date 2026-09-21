package main

import (
	pb "GoKeyMux/proto"
	"context"
	"errors"
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
}

func (s *GoKeyMuxService) KeyService(stream pb.RpcKeyService_KeyServiceServer) error {
	for {
		req, err := stream.Recv()
		// TODO
		if err == io.EOF {
			return stream.SendAndClose(&pb.KeyReturn{
				IsAllDone: false, // TODO: 错误计数器？必须？
				MetaData:  time.Now().String(),
			})
		}
		if err != nil {
			return err
		}
		slog.Debug("Received key request", "key", req.GetKey(), "isPressed", req.GetIsPressed(), "isRune", req.GetIsRune())
	}
}

func (s *GoKeyMuxService) KeyServiceDebug(ctx context.Context, req *pb.KeyInputDebug) (*pb.KeyReturnDebug, error) {
	// TODO
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return &pb.KeyReturnDebug{
			IsFinished: false,
			Key:        req.GetKey(),
			TimeStamp:  time.Now().String(),
		}, nil
	}
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

func StartService() (*grpc.Server, <-chan error, error) {
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
	pb.RegisterRpcKeyServiceServer(server, &GoKeyMuxService{})
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
