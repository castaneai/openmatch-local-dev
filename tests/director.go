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

func (d *Director) AssignTickets(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		// https://github.com/googleforgames/open-match/issues/1240#issuecomment-769898964
		if match.AllocateGameserver {
			gs := allocateGameServer(d.omFrontend)
			as := &pb.Assignment{
				Connection: string(gs.ConnectionName()),
			}
			asgs = append(asgs, &pb.AssignmentGroup{
				TicketIds:  ticketIDs(match),
				Assignment: as,
			})
			if match.Backfill != nil {
				gs.StartBackfill(match.Backfill, as)
			}
		} else {
			// AssignTickets does nothing;
			// wait for the Assignment to be conveyed by AcknowledgeBackfill.
		}
	}
	if _, err := d.omBackend.AssignTickets(ctx, &pb.AssignTicketsRequest{Assignments: asgs}); err != nil {
		return nil, fmt.Errorf("failed to assign tickets: %w", err)
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
