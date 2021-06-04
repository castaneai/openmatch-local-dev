package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"open-match.dev/open-match/pkg/pb"
)

func TestMakeMatches(t *testing.T) {
	pool := &pb.Pool{
		Name: "test-pool",
	}
	profile := &pb.MatchProfile{
		Name:  "test-profile",
		Pools: []*pb.Pool{pool},
	}

	t.Run("remaining tickets will make match with backfill", func(t *testing.T) {
		poolTickets := map[string][]*pb.Ticket{
			pool.Name: {
				&pb.Ticket{Id: "ticket-1"},
				&pb.Ticket{Id: "ticket-2"},
			},
		}
		poolBackfills := map[string][]*pb.Backfill{}
		matches, err := makeMatches(profile, poolTickets, poolBackfills)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Len(t, matches[0].Tickets, len(poolTickets[pool.Name]))
		assert.NotNil(t, matches[0].Backfill)
		assert.True(t, matches[0].AllocateGameserver)
	})

	t.Run("fulfilled tickets will make full-match without backfill", func(t *testing.T) {
		poolTickets := map[string][]*pb.Ticket{
			pool.Name: {
				&pb.Ticket{Id: "ticket-1"},
				&pb.Ticket{Id: "ticket-2"},
				&pb.Ticket{Id: "ticket-3"},
			},
		}
		poolBackfills := map[string][]*pb.Backfill{}
		matches, err := makeMatches(profile, poolTickets, poolBackfills)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Len(t, matches[0].Tickets, len(poolTickets[pool.Name]))
		assert.Nil(t, matches[0].Backfill)
		assert.True(t, matches[0].AllocateGameserver)
	})

	t.Run(" tickets will make match without backfill", func(t *testing.T) {
		poolTickets := map[string][]*pb.Ticket{
			pool.Name: {
				&pb.Ticket{Id: "ticket-1"},
			},
		}
		numTickets := len(poolTickets[pool.Name])
		poolBackfills := map[string][]*pb.Backfill{}
		matches, err := makeMatches(profile, poolTickets, poolBackfills)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Len(t, matches[0].Tickets, numTickets)
		assert.Equal(t, "ticket-1", matches[0].Tickets[0].Id)
		assert.NotNil(t, matches[0].Backfill)
		assert.True(t, matches[0].AllocateGameserver)

		poolBackfills[pool.Name] = nil
		poolBackfills[pool.Name] = append(poolBackfills[pool.Name], matches[0].Backfill)

		poolTickets = map[string][]*pb.Ticket{
			pool.Name: {
				&pb.Ticket{Id: "ticket-2"},
			},
		}
		numTickets = len(poolTickets[pool.Name])

		matches, err = makeMatches(profile, poolTickets, poolBackfills)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Len(t, matches[0].Tickets, numTickets)
		assert.NotNil(t, matches[0].Backfill)
		assert.False(t, matches[0].AllocateGameserver)
	})

}
