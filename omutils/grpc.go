package omutils

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/pb"
)

func NewOMFrontendClient(addr string) (pb.FrontendServiceClient, error) {
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	cc, err := grpc.Dial(addr, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to open match frontend: %w", err)
	}
	return pb.NewFrontendServiceClient(cc), nil
}

func NewOMBackendClient(addr string) (pb.BackendServiceClient, error) {
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	cc, err := grpc.Dial(addr, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to open match backend: %w", err)
	}
	return pb.NewBackendServiceClient(cc), nil
}
