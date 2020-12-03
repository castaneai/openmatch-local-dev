package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"
)

const (
	// A match function is in omdemo namespace
	// see ../matchfunction/main.go
	// see also https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records
	matchFunctionHost = "matchfunction.omdemo.svc.cluster.local."
	matchFunctionPort = 50502
)

func main() {
	// See portForward section in skaffold.yaml
	omBackendAddr := "localhost:50505"
	omBackend, err := newOMBackendClient(omBackendAddr)
	if err != nil {
		log.Fatalf("failed to connect to open-match backend: %+v", err)
	}
	mfConfig := &pb.FunctionConfig{
		Host: matchFunctionHost,
		Port: matchFunctionPort,
		Type: pb.FunctionConfig_GRPC,
	}

	ctx := context.Background()

	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		matches, err := fetchMatches(ctx, omBackend, mfConfig)
		if err != nil {
			log.Fatalf("failed to fetch matches: %+v", err)
		}

		if len(matches) < 1 {
			continue
		}
		log.Printf("%d matches found", len(matches))

		for _, match := range matches {
			if err := assignTickets(ctx, omBackend, match); err != nil {
				log.Fatalf("failed to assign tickets: %+v", err)
			}
		}
	}
}

func fetchMatches(ctx context.Context, omBackend pb.BackendServiceClient, config *pb.FunctionConfig) ([]*pb.Match, error) {
	stream, err := omBackend.FetchMatches(ctx, &pb.FetchMatchesRequest{Config: config, Profile: &pb.MatchProfile{Name: "test", Pools: []*pb.Pool{
		{Name: "pool-A"},
		{Name: "pool-B"},
	}}})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch matches: %+v", err)
	}

	var matches []*pb.Match
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to recv matches: %+v", err)
		}
		matches = append(matches, resp.Match)
	}
	return matches, nil
}

func assignTickets(ctx context.Context, omBackend pb.BackendServiceClient, match *pb.Match) error {
	var ticketIDs []string
	for _, ticket := range match.Tickets {
		ticketIDs = append(ticketIDs, ticket.Id)
	}
	asg := &pb.AssignmentGroup{
		Assignment: &pb.Assignment{Connection: "dummy"},
		TicketIds:  ticketIDs,
	}
	resp, err := omBackend.AssignTickets(ctx, &pb.AssignTicketsRequest{Assignments: []*pb.AssignmentGroup{asg}})
	if err != nil {
		return err
	}
	for _, failure := range resp.Failures {
		log.Printf("assign tickets failure: %+v", failure)
	}
	return nil
}

func newOMBackendClient(addr string) (pb.BackendServiceClient, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return pb.NewBackendServiceClient(cc), nil
}
