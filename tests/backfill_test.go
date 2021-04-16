package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"

	"open-match.dev/open-match/pkg/pb"
)

const (
	playersPerMatch = 3
	openSlotsKey    = "openSlots"
)

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
	return playersPerMatch, nil
}

func TestCreateTicketWithBackfill(t *testing.T) {
	frontend := newOMFrontendClient(t)
	backend := newOMBackendClient(t)

	profile := &pb.MatchProfile{Name: "test-profile", Pools: []*pb.Pool{
		{Name: "test-pool", CreatedAfter: timestamppb.New(time.Now())},
	}}

	ticket1 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches := fetchMatches(t, backend, profile)
		assert.Len(t, matches, 1)
		assert.Equal(t, true, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.Equal(t, ticket1.Id, matches[0].Tickets[0].Id)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(playersPerMatch-1), openSlots)

		mustAssignTickets(t, backend, matches[0], "test-gs")
		assignment := mustAssignment(t, frontend, ticket1.Id, 3*time.Second)
		assert.Equal(t, "test-gs", assignment.Connection)

		// The allocated GameServer starts polling Open Match to acknowledge the backfill
		// ref: https://open-match.dev/site/docs/guides/backfill/
		ctx, cancelBackfill := context.WithCancel(context.Background())
		defer cancelBackfill()
		go func(bf *pb.Backfill, as *pb.Assignment) {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					mustAcknowledgeBackfill(t, frontend, bf, as)
				}
			}
		}(matches[0].Backfill, assignment)
	}

	ticket2 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches := fetchMatches(t, backend, profile)
		assert.Len(t, matches, 1)
		assert.Equal(t, false, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(playersPerMatch-2), openSlots)

		assignment := mustAssignment(t, frontend, ticket2.Id, 3*time.Second)
		assert.Equal(t, "test-gs", assignment.Connection)
	}
}
