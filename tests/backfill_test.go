package tests

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stretchr/testify/assert"

	"open-match.dev/open-match/pkg/pb"
)

func TestCreateTicketWithBackfill(t *testing.T) {
	frontend := newOMFrontendClient(t)
	backend := newOMBackendClient(t)

	profile := &pb.MatchProfile{Name: "test-profile", Pools: []*pb.Pool{
		{Name: "test-pool", CreatedAfter: timestamppb.New(time.Now())},
	}}

	var allocatedGameServer *GameServer

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

		allocatedGameServer = AllocateGameServer("test-gs", frontend)

		mustAssignTickets(t, backend, matches[0], allocatedGameServer.ConnectionName())
		assignment := mustAssignment(t, frontend, ticket1.Id, 3*time.Second)

		allocatedGameServer.Connect(ticket1.Id)
		allocatedGameServer.StartBackfillCreated(matches[0].Backfill, assignment)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)
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
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)

		allocatedGameServer.Connect(ticket2.Id)
	}

	ticket3 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches := fetchMatches(t, backend, profile)
		assert.Len(t, matches, 1)
		assert.Equal(t, false, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(0), openSlots)

		assignment := mustAssignment(t, frontend, ticket3.Id, 3*time.Second)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)

		allocatedGameServer.Connect(ticket3.Id)
	}
}
