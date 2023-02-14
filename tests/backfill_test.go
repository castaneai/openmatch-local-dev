package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/castaneai/openmatch-local-dev/omutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"open-match.dev/open-match/pkg/pb"
)

func TestCreateTicketWithBackfill(t *testing.T) {
	ctx := context.Background()
	frontend := newOMFrontendClient(t)
	backend := newOMBackendClient(t)
	director := &Director{
		omFrontend: frontend,
		omBackend:  backend,
	}

	profile := &pb.MatchProfile{Name: "test-profile", Pools: []*pb.Pool{
		// Create a unique pool name for each test to avoid mixing with tickets created during previous tests.
		{Name: fmt.Sprintf("test-pool-%s", uuid.Must(uuid.NewRandom()))},
	}}

	var allocatedGameServer *GameServer
	var assignment *pb.Assignment

	ticket1 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches, err := director.FetchMatches(ctx, profile, mfConfig)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, true, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.Equal(t, ticket1.Id, matches[0].Tickets[0].Id)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := omutils.GetOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(omutils.PlayersPerMatch-1), openSlots)

		_, err = director.AssignTickets(ctx, matches)
		assert.NoError(t, err)

		assignment = mustAssignment(t, frontend, ticket1.Id, 3*time.Second)
		gs, ok := getGameServer(GameServerConnectionName(assignment.Connection))
		assert.True(t, ok)
		allocatedGameServer = gs
		assert.NoError(t, allocatedGameServer.ConnectPlayer(ctx, ticket1.Id))
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)
	}

	ticket2 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches, err := director.FetchMatches(ctx, profile, mfConfig)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, false, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := omutils.GetOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(omutils.PlayersPerMatch-2), openSlots)

		_, err = director.AssignTickets(ctx, matches)
		assert.NoError(t, err)

		assignment := mustAssignment(t, frontend, ticket2.Id, 3*time.Second)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)

		assert.NoError(t, allocatedGameServer.ConnectPlayer(ctx, ticket2.Id))
	}

	ticket3 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches, err := director.FetchMatches(ctx, profile, mfConfig)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, false, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := omutils.GetOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(omutils.PlayersPerMatch-3), openSlots)

		_, err = director.AssignTickets(ctx, matches)
		assert.NoError(t, err)

		assignment := mustAssignment(t, frontend, ticket3.Id, 3*time.Second)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)

		assert.NoError(t, allocatedGameServer.ConnectPlayer(ctx, ticket3.Id))
	}

	assert.NoError(t, allocatedGameServer.StopBackfill())
	assert.NoError(t, allocatedGameServer.DisconnectPlayer(ctx, ticket1.Id))
	bf, err := allocatedGameServer.CreateBackfill(ctx, 1)
	assert.NoError(t, err)
	allocatedGameServer.StartBackfill(bf, assignment)

	ticket4 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches, err := director.FetchMatches(ctx, profile, mfConfig)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, false, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := omutils.GetOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(omutils.PlayersPerMatch-3), openSlots)

		_, err = director.AssignTickets(ctx, matches)
		assert.NoError(t, err)

		assignment := mustAssignment(t, frontend, ticket4.Id, 3*time.Second)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)
		assert.NoError(t, allocatedGameServer.ConnectPlayer(ctx, ticket4.Id))
	}
}
