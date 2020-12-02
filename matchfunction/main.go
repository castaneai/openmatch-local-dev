package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"open-match.dev/open-match/pkg/matchfunction"

	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"
)

func main() {
	// A query service is in open-match core namespace
	// see https://github.com/googleforgames/open-match/blob/26d1aa236a5238b1387e91d506d21ed09f3891cc/install/helm/open-match/values.yaml#L54
	// see also https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records
	qsAddr := "om-query.open-match.svc.cluster.local.:50503"
	qsc, err := newQueryServiceClient(qsAddr)
	if err != nil {
		log.Fatalf("failed to connect to QueryService: %+v", err)
	}

	addr := ":50502"
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %+v", err)
	}
	s := grpc.NewServer()
	pb.RegisterMatchFunctionServer(s, &matchFunctionService{qsc: qsc})

	log.Printf("litening on %s...", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %+v", err)
	}
}

type matchFunctionService struct {
	qsc pb.QueryServiceClient
}

func (s *matchFunctionService) Run(request *pb.RunRequest, stream pb.MatchFunction_RunServer) error {
	poolTickets, err := matchfunction.QueryPools(stream.Context(), s.qsc, request.Profile.Pools)
	if err != nil {
		log.Printf("failed to query pools: %+v", err)
		return err
	}
	matches, err := makeMatches(poolTickets)
	if err != nil {
		log.Printf("failed to make matches: %+v", err)
		return err
	}
	for _, match := range matches {
		if err := stream.Send(&pb.RunResponse{Proposal: match}); err != nil {
			log.Printf("failed to send match proposal: %+v", err)
			return err
		}
	}
	return nil
}

func newQueryServiceClient(addr string) (pb.QueryServiceClient, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return pb.NewQueryServiceClient(cc), nil
}

const (
	matchName = "omdemo-example-match"
)

// Copied from https://github.com/googleforgames/open-match/blob/26d1aa236a5238b1387e91d506d21ed09f3891cc/examples/functions/golang/soloduel/mmf/matchfunction.go
func makeMatches(poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error) {
	tickets := map[string]*pb.Ticket{}
	for key, pool := range poolTickets {
		log.Printf("pool[%s]: %d ticket(s)", key, len(pool))
		for _, ticket := range pool {
			tickets[ticket.GetId()] = ticket
		}
	}

	var matches []*pb.Match

	t := time.Now().Format("2006-01-02T15:04:05.00")

	thisMatch := make([]*pb.Ticket, 0, 2)
	matchNum := 0

	for _, ticket := range tickets {
		thisMatch = append(thisMatch, ticket)

		if len(thisMatch) >= 2 {
			matches = append(matches, &pb.Match{
				MatchId:       fmt.Sprintf("profile-%s-time-%s-num-%d", matchName, t, matchNum),
				MatchProfile:  matchName,
				MatchFunction: matchName,
				Tickets:       thisMatch,
			})

			thisMatch = make([]*pb.Ticket, 0, 2)
			matchNum++
		}
	}

	return matches, nil
}
