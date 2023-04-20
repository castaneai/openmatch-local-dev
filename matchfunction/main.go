package main

import (
	"fmt"
	"log"
	"net"

	"github.com/castaneai/openmatch-local-dev/omutils"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"open-match.dev/open-match/pkg/matchfunction"
	"open-match.dev/open-match/pkg/pb"
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
	poolBackfills, err := matchfunction.QueryBackfillPools(stream.Context(), s.qsc, request.Profile.Pools)
	if err != nil {
		log.Printf("failed to query backfill pools: %+v", err)
		return err
	}
	for poolName, tickets := range poolTickets {
		if len(tickets) > 0 {
			log.Printf("pool: %s, tickets: %s", poolName, ticketIDs(tickets))
		}
	}

	matches, err := makeMatches(request.Profile, poolTickets, poolBackfills)
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

func makeMatches(profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket, poolBackfills map[string][]*pb.Backfill) ([]*pb.Match, error) {
	var matches []*pb.Match

	// First, creating matches with the existing backfills.
	for pool, tickets := range poolTickets {
		var backfills []*pb.Backfill
		bs, ok := poolBackfills[pool]
		if ok {
			backfills = bs
		}

		newMatches, remainingTickets, err := handleBackfills(profile, tickets, backfills)
		if err != nil {
			return nil, err
		}
		matches = append(matches, newMatches...)

		// Second, creating full-matches with tickets
		newMatches, remainingTickets = makeFullMatches(profile, remainingTickets)
		matches = append(matches, newMatches...)

		if len(remainingTickets) > 0 {
			// Third, the remaining tickets will make matches with backfill
			remainingMatch, err := makeMatchWithBackfill(profile, remainingTickets)
			if err != nil {
				return nil, err
			}
			matches = append(matches, remainingMatch)
		}
	}

	return matches, nil
}

func makeFullMatches(profile *pb.MatchProfile, tickets []*pb.Ticket) ([]*pb.Match, []*pb.Ticket) {
	var matches []*pb.Match
	for len(tickets) >= omutils.PlayersPerMatch {
		match := newMatch(profile, tickets[:omutils.PlayersPerMatch], nil)
		match.AllocateGameserver = true
		tickets = tickets[omutils.PlayersPerMatch:]
		matches = append(matches, match)
	}
	return matches, tickets
}

func handleBackfills(profile *pb.MatchProfile, tickets []*pb.Ticket, backfills []*pb.Backfill) ([]*pb.Match, []*pb.Ticket, error) {
	var matches []*pb.Match

	for _, backfill := range backfills {
		openSlots, err := omutils.GetOpenSlots(backfill)
		if err != nil {
			return nil, nil, err
		}

		var matchTickets []*pb.Ticket
		for openSlots > 0 && len(tickets) > 0 {
			matchTickets = append(matchTickets, tickets[0])
			tickets = tickets[1:]
			openSlots--
		}

		if len(matchTickets) > 0 {
			if err := omutils.SetOpenSlots(backfill, openSlots); err != nil {
				return nil, nil, err
			}
			matches = append(matches, newMatch(profile, matchTickets, backfill))
		}
	}
	return matches, tickets, nil
}

func makeMatchWithBackfill(profile *pb.MatchProfile, tickets []*pb.Ticket) (*pb.Match, error) {
	if len(tickets) == 0 {
		return nil, fmt.Errorf("tickets are required")
	}
	if len(tickets) > omutils.PlayersPerMatch {
		return nil, fmt.Errorf("too many tickets")
	}
	backfill, err := newBackfill(newSearchFields(), omutils.PlayersPerMatch-len(tickets))
	if err != nil {
		return nil, err
	}
	match := newMatch(profile, tickets, backfill)
	match.AllocateGameserver = true
	return match, nil
}

func newSearchFields() *pb.SearchFields {
	return &pb.SearchFields{}
}

func newMatch(profile *pb.MatchProfile, tickets []*pb.Ticket, backfill *pb.Backfill) *pb.Match {
	return &pb.Match{
		MatchId:       fmt.Sprintf("%s-%s", profile.Name, uuid.Must(uuid.NewRandom())),
		MatchProfile:  profile.Name,
		MatchFunction: "test",
		Tickets:       tickets,
		Backfill:      backfill,
	}
}

func newBackfill(searchFields *pb.SearchFields, openSlots int) (*pb.Backfill, error) {
	b := &pb.Backfill{
		SearchFields: searchFields,
		CreateTime:   timestamppb.Now(),
		Generation:   0,
	}
	if err := omutils.SetOpenSlots(b, int32(openSlots)); err != nil {
		return nil, err
	}
	return b, nil
}
