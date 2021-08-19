package main

import (
	"fmt"
	"log"
	"net"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/matchfunction"
	"open-match.dev/open-match/pkg/pb"
)

const (
	playersPerMatch = 3
	openSlotsKey    = "openSlots"
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
	log.Printf("start query pools (profile: %s, pools: %v)", request.Profile.Name, poolNames)

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
	if poolTicketsIsEmpty(poolTickets) {
		log.Printf("query pools result empty (profile: %s, pools: %v)", request.Profile.Name, poolNames)
		return nil
	}
	log.Printf("%d pool tickets found with profile: %s", len(poolTickets), request.Profile.Name)
	for poolName, tickets := range poolTickets {
		log.Printf("pool: %s, tickets: %s", poolName, spew.Sdump(tickets))
	}
	log.Printf("%d backfills found with profile: %s", len(poolBackfills), request.Profile.Name)
	for poolName, backfills := range poolBackfills {
		log.Printf("pool: %s, backfills: %s", poolName, spew.Sdump(backfills))
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

func makeMatches(profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket, poolBackfills map[string][]*pb.Backfill) ([]*pb.Match, error) {
	tickets := allTickets(poolTickets)
	backfills := allBackfills(poolBackfills)

	var matches []*pb.Match

	// First, creating matches with the existing backfills.
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

	return matches, nil
}

func makeFullMatches(profile *pb.MatchProfile, tickets []*pb.Ticket) ([]*pb.Match, []*pb.Ticket) {
	var matches []*pb.Match
	for len(tickets) >= playersPerMatch {
		match := newMatch(profile, tickets[:playersPerMatch], nil)
		match.AllocateGameserver = true
		tickets = tickets[playersPerMatch:]
		matches = append(matches, match)
	}
	return matches, tickets
}

func allTickets(poolTickets map[string][]*pb.Ticket) []*pb.Ticket {
	allTicketsMap := map[string]*pb.Ticket{}
	for _, tickets := range poolTickets {
		for _, ticket := range tickets {
			allTicketsMap[ticket.Id] = ticket
		}
	}
	var allTickets []*pb.Ticket
	for _, ticket := range allTicketsMap {
		allTickets = append(allTickets, ticket)
	}
	return allTickets
}

func allBackfills(poolBackfills map[string][]*pb.Backfill) []*pb.Backfill {
	allBackfillsMap := map[string]*pb.Backfill{}
	for _, backfills := range poolBackfills {
		for _, backfill := range backfills {
			allBackfillsMap[backfill.Id] = backfill
		}
	}
	var allBackfills []*pb.Backfill
	for _, backfill := range allBackfillsMap {
		allBackfills = append(allBackfills, backfill)
	}
	return allBackfills
}

func handleBackfills(profile *pb.MatchProfile, tickets []*pb.Ticket, backfills []*pb.Backfill) ([]*pb.Match, []*pb.Ticket, error) {
	var matches []*pb.Match

	for _, backfill := range backfills {
		openSlots, err := getOpenSlots(backfill)
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
			if err := setOpenSlots(backfill, openSlots); err != nil {
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
	if len(tickets) > playersPerMatch {
		return nil, fmt.Errorf("too many tickets")
	}
	backfill, err := newBackfill(newSearchFields(), playersPerMatch-len(tickets))
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
		CreateTime:   ptypes.TimestampNow(),
		Generation:   0,
	}
	if err := setOpenSlots(b, int32(openSlots)); err != nil {
		return nil, err
	}
	return b, nil
}

func setOpenSlots(b *pb.Backfill, val int32) error {
	if b.Extensions == nil {
		b.Extensions = make(map[string]*any.Any)
	}
	any, err := ptypes.MarshalAny(&wrappers.Int32Value{Value: val})
	if err != nil {
		return err
	}
	b.Extensions[openSlotsKey] = any
	return nil
}

func getOpenSlots(b *pb.Backfill) (int32, error) {
	if b == nil {
		return 0, fmt.Errorf("expected backfill is not nil")
	}
	if b.Extensions != nil {
		if any, ok := b.Extensions[openSlotsKey]; ok {
			var val wrappers.Int32Value
			err := ptypes.UnmarshalAny(any, &val)
			if err != nil {
				return 0, err
			}

			return val.Value, nil
		}
	}
	// defaults to zero
	return 0, nil
}
