package tests

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/pb"
)

const (
	// see ../matchfunction/main.go
	// see also https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records
	matchFunctionHost = "matchfunction.open-match.svc.cluster.local."
	matchFunctionPort = 50502

	// See portForward section in skaffold.yaml
	frontendAddr = "localhost:50504"
	backendAddr  = "localhost:50505"
)

var mfConfig = &pb.FunctionConfig{
	Host: matchFunctionHost,
	Port: matchFunctionPort,
	Type: pb.FunctionConfig_GRPC,
}

func newOMFrontendClient(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(frontendAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	return pb.NewFrontendServiceClient(cc)
}

func newOMBackendClient(t *testing.T) pb.BackendServiceClient {
	cc, err := grpc.Dial(backendAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	return pb.NewBackendServiceClient(cc)
}

func mustCreateTicket(t *testing.T, fe pb.FrontendServiceClient, ticket *pb.Ticket) *pb.Ticket {
	t.Helper()
	rt, err := fe.CreateTicket(context.Background(), &pb.CreateTicketRequest{
		Ticket: ticket,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

func mustAssignment(t *testing.T, fe pb.FrontendServiceClient, ticketID string, timeout time.Duration) *pb.Assignment {
	t.Helper()
	as, err := waitForAssignment(fe, ticketID, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

func waitForAssignment(fe pb.FrontendServiceClient, ticketID string, timeout time.Duration) (*pb.Assignment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stream, err := fe.WatchAssignments(ctx, &pb.WatchAssignmentsRequest{TicketId: ticketID})
	if err != nil {
		return nil, err
	}
	for {
		resp, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		return resp.Assignment, nil
	}
}
