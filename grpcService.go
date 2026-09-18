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
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

type GoKeyMuxService struct {
	pb.UnimplementedRpcKeyServiceServer
}

func (s *GoKeyMuxService) KeyService(stream pb.RpcKeyService_KeyServiceServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.KeyReturn{
				IsAllDone: false,
				MetaData:  "Service Unavailable",
			})
		}
		if err != nil {
			return err
		}
		slog.Debug("Received key request", "key", req.GetKey(), "isPressed", req.GetIsPressed(), "isRune", req.GetIsRune())
	}
}

func (s *GoKeyMuxService) KeyServiceDebug(ctx context.Context, req *pb.KeyInputDebug) (*pb.KeyReturnDebug, error) {
	slog.Debug("Received key request", "req", req)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return &pb.KeyReturnDebug{
			IsFinished: false,
			Key:        req.GetKey(),
			TimeStamp:  req.GetTimeStamp(),
		}, nil
	}
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
			Time:              2 * time.Second,
			Timeout:           1 * time.Second,
			MaxConnectionIdle: 120 * time.Second, // 空闲后关闭连接
		}),
		// 服务端 enforcement：限制客户端 ping 频率与无流 ping。
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: false,
		}),
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
