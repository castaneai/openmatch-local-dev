package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/davecgh/go-spew/spew"

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
	var poolNames []string
	for _, pool := range request.Profile.Pools {
		poolNames = append(poolNames, pool.Name)
	}
	log.Printf("start query pools (profile: %s, pools: %v)", request.Profile.Name, poolNames)

	poolTickets, err := matchfunction.QueryPools(stream.Context(), s.qsc, request.Profile.Pools)
	if err != nil {
		log.Printf("failed to query pools: %+v", err)
		return err
	}
	if poolTicketsIsEmpty(poolTickets) {
		log.Printf("query pools result empty (profile: %s, pools: %v)", request.Profile.Name, poolNames)
		return nil
	}
	log.Printf("%d pool tickets found with profile: %s", len(poolTickets), request.Profile.Name)
	for poolName, tickets := range poolTickets {
		log.Printf("pool: %s, tickets: %s", poolName, spew.Sdump(tickets))
	}
	matches, err := makeMatches(poolTickets, request.Profile)
	if err != nil {
		log.Printf("failed to make matches: %+v", err)
		return err
	}
	for _, match := range matches {
		if err := stream.Send(&pb.RunResponse{Proposal: match}); err != nil {
			log.Printf("failed to send match proposal: %+v", err)
			return err
		}
		log.Printf("sent match proposal: %s", spew.Sdump(match))
	}
	return nil
}

func poolTicketsIsEmpty(poolTickets map[string][]*pb.Ticket) bool {
	if len(poolTickets) < 1 {
		return true
	}
	for _, tickets := range poolTickets {
		if len(tickets) > 0 {
			return false
		}
	}
	return true
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
func makeMatches(poolTickets map[string][]*pb.Ticket, profile *pb.MatchProfile) ([]*pb.Match, error) {
	tickets := map[string]*pb.Ticket{}
	for _, pool := range poolTickets {
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
				MatchProfile:  profile.Name,
				MatchFunction: matchName,
				Tickets:       thisMatch,
			})

			thisMatch = make([]*pb.Ticket, 0, 2)
			matchNum++
		}
	}

	return matches, nil
}
