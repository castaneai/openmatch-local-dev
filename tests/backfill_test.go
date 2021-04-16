package tests

import (
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
	profile := &pb.MatchProfile{Name: "test-profile", Pools: []*pb.Pool{
		{Name: "test-pool", CreatedAfter: timestamppb.New(time.Now())},
	}}
	fe := newOMFrontendClient(t)
	be := newOMBackendClient(t)
	mustCreateTicket(t, fe, &pb.Ticket{SearchFields: &pb.SearchFields{Tags: []string{"test"}}})
	matches := fetchMatches(t, be, profile)
	assert.Len(t, matches, 1)
	assert.Len(t, matches[0].Tickets, 1)
}
