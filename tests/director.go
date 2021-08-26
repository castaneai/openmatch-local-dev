package tests

import (
	"context"
	"fmt"
	"io"

	"open-match.dev/open-match/pkg/pb"
)

type Director struct {
	omFrontend pb.FrontendServiceClient
	omBackend  pb.BackendServiceClient
}

func (d *Director) FetchMatches(ctx context.Context, profile *pb.MatchProfile, mfConfig *pb.FunctionConfig) ([]*pb.Match, error) {
	stream, err := d.omBackend.FetchMatches(ctx, &pb.FetchMatchesRequest{Config: mfConfig, Profile: profile})
	if err != nil {
		return nil, err
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

type AssignResult struct {
	AllocatedServer  *GameServer
	AssignmentGroups []*pb.AssignmentGroup
}

func (d *Director) AssignTickets(ctx context.Context, matches []*pb.Match) (*AssignResult, error) {
	var gs *GameServer
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		// https://github.com/googleforgames/open-match/issues/1240#issuecomment-769898964
		if match.Backfill == nil {
			gs = AllocateGameServer(d.omFrontend)
			asgs = append(asgs, &pb.AssignmentGroup{
				TicketIds: ticketIDs(match),
				Assignment: &pb.Assignment{
					Connection: string(gs.ConnectionName()),
				},
			})
		} else if match.AllocateGameserver {
			gs = AllocateGameServer(d.omFrontend)
			gs.StartBackfillCreated(match.Backfill, &pb.Assignment{
				Connection: string(gs.ConnectionName()),
			})
		}
	}
	if _, err := d.omBackend.AssignTickets(ctx, &pb.AssignTicketsRequest{Assignments: asgs}); err != nil {
		return nil, fmt.Errorf("failed to assign tickets: %w", err)
	}
	return &AssignResult{
		AllocatedServer:  gs,
		AssignmentGroups: asgs,
	}, nil
}

func ticketIDs(match *pb.Match) []string {
	var ids []string
	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
