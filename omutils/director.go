package omutils

import (
	"context"

	"github.com/castaneai/omtools"
	"github.com/google/uuid"
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
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  ticketIDs(match),
			Assignment: &pb.Assignment{Connection: uuid.Must(uuid.NewRandom()).String()},
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
