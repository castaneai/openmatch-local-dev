package omutils

import (
	"context"
	"log"

	"github.com/bojand/hri"
	"github.com/castaneai/omtools"
	"open-match.dev/open-match/pkg/pb"
)

func NewTestDirector(backendAddr string, profile *pb.MatchProfile) (*omtools.Director, error) {
	backend, err := NewOMBackendClient(backendAddr)
	if err != nil {
		return nil, err
	}
	return omtools.NewDirector(backend, profile, &pb.FunctionConfig{
		Host: "matchfunction.open-match.svc.cluster.local.",
		Port: 50502,
		Type: pb.FunctionConfig_GRPC,
	}, assignFunc(dummyAssign)), nil
}

type assignFunc func(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)

func (f assignFunc) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	return f(ctx, matches)
}

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		log.Printf("assign '%s' to tickets: %v", conn, tids)
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  tids,
			Assignment: &pb.Assignment{Connection: conn},
		})
	}
	return asgs, nil
}

func ticketIDs(match *pb.Match) []string {
	var ids []string
	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
