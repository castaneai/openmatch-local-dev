package main

import (
	"fmt"
	"log"
	"net"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/matchfunction"
	"open-match.dev/open-match/pkg/pb"
)

const (
	playersPerMatch = 2
)

func main() {
	// A query service is in open-match core namespace
	// see https://github.com/googleforgames/open-match/blob/26d1aa236a5238b1387e91d506d21ed09f3891cc/install/helm/open-match/values.yaml#L54
	// see also https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#a-aaaa-records
	qsAddr := "open-match-query.open-match.svc.cluster.local.:50503"
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
	var poolNames []string
	for _, pool := range request.Profile.Pools {
		poolNames = append(poolNames, pool.Name)
	}

	poolTickets, err := matchfunction.QueryPools(stream.Context(), s.qsc, request.Profile.Pools)
	if err != nil {
		log.Printf("failed to query pools: %+v", err)
		return err
	}
	for poolName, tickets := range poolTickets {
		if len(tickets) > 0 {
			log.Printf("pool: %s, tickets: %s", poolName, ticketIDs(tickets))
		}
	}

	var matches []*pb.Match
	for _, tickets := range poolTickets {
		ms, err := makeMatches(request.Profile, tickets)
		if err != nil {
			log.Printf("failed to make matches: %+v", err)
			return err
		}
		matches = append(matches, ms...)
	}
	for _, match := range matches {
		if err := stream.Send(&pb.RunResponse{Proposal: match}); err != nil {
			log.Printf("failed to send match proposal: %+v", err)
			return err
		}
	}
	if len(matches) > 0 {
		log.Printf("sent %d match proposal(s)", len(matches))
	}
	return nil
}

func newQueryServiceClient(addr string) (pb.QueryServiceClient, error) {
	cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return pb.NewQueryServiceClient(cc), nil
}

func ticketIDs(ts []*pb.Ticket) []string {
	var tids []string
	for _, t := range ts {
		tids = append(tids, t.Id)
	}
	return tids
}

func makeMatches(profile *pb.MatchProfile, tickets []*pb.Ticket) ([]*pb.Match, error) {
	var matches []*pb.Match
	for len(tickets) >= playersPerMatch {
		match := newMatch(profile, tickets[:playersPerMatch])
		match.AllocateGameserver = true
		tickets = tickets[playersPerMatch:]
		matches = append(matches, match)
	}
	return matches, nil
}

func newMatch(profile *pb.MatchProfile, tickets []*pb.Ticket) *pb.Match {
	return &pb.Match{
		MatchId:       fmt.Sprintf("%s-%s", profile.Name, uuid.Must(uuid.NewRandom())),
		MatchProfile:  profile.Name,
		MatchFunction: "test",
		Tickets:       tickets,
	}
}
