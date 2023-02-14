package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
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

var errAssignmentTimeout = errors.New("wait assignment timeout")
var mfConfig = &pb.FunctionConfig{
	Host: matchFunctionHost,
	Port: matchFunctionPort,
	Type: pb.FunctionConfig_GRPC,
}

func newOMFrontendClient(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(frontendAddr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	return pb.NewFrontendServiceClient(cc)
}

func newOMBackendClient(t *testing.T) pb.BackendServiceClient {
	cc, err := grpc.Dial(backendAddr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	return pb.NewBackendServiceClient(cc)
}

func newPool(name string) *pb.Pool {
	return &pb.Pool{
		Name:         name,
		CreatedAfter: timestamppb.New(time.Now().Add(-1000 * time.Millisecond)),
	}
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
	ctx := context.Background()
	stream, err := fe.WatchAssignments(ctx, &pb.WatchAssignmentsRequest{TicketId: ticketID})
	if err != nil {
		return nil, err
	}

	resch := make(chan struct {
		resp *pb.WatchAssignmentsResponse
		err  error
	})
	go func() {
		for {
			resp, err := stream.Recv()
			resch <- struct {
				resp *pb.WatchAssignmentsResponse
				err  error
			}{resp: resp, err: err}
		}
	}()
	for {
		select {
		case res := <-resch:
			if res.err != nil {
				return nil, fmt.Errorf("failed to recv stream on watch assignment: %+v", res.err)
			}
			if res.resp.Assignment != nil {
				return res.resp.Assignment, nil
			}
		case <-time.After(timeout):
			return nil, errAssignmentTimeout
		}
	}
}
