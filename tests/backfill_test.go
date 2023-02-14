package tests

import (
	"context"
	"testing"
	"time"

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
		newPool("test-pool"),
	}}

	var allocatedGameServer *GameServer
	var assignment *pb.Assignment
	var currentBackfill *pb.Backfill

	ticket1 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches, err := director.FetchMatches(ctx, profile, mfConfig)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, true, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.Equal(t, ticket1.Id, matches[0].Tickets[0].Id)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(playersPerMatch-1), openSlots)
		currentBackfill = matches[0].Backfill

		res, err := director.AssignTickets(ctx, matches)
		assert.NoError(t, err)
		assert.NotNil(t, res.AllocatedServer)
		allocatedGameServer = res.AllocatedServer
		assert.Len(t, res.AssignmentGroups, 0)

		assignment = mustAssignment(t, frontend, ticket1.Id, 3*time.Second)
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
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(playersPerMatch-2), openSlots)

		res, err := director.AssignTickets(ctx, matches)
		assert.NoError(t, err)
		assert.Nil(t, res.AllocatedServer)
		assert.Len(t, res.AssignmentGroups, 0)

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
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(playersPerMatch-3), openSlots)

		res, err := director.AssignTickets(ctx, matches)
		assert.NoError(t, err)
		assert.Nil(t, res.AllocatedServer)
		assert.Len(t, res.AssignmentGroups, 0)

		assignment := mustAssignment(t, frontend, ticket3.Id, 3*time.Second)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)

		assert.NoError(t, allocatedGameServer.ConnectPlayer(ctx, ticket3.Id))
	}

	assert.NoError(t, allocatedGameServer.StopBackfill(ctx, currentBackfill.Id))
	assert.NoError(t, allocatedGameServer.DisconnectPlayer(ctx, ticket1.Id))
	assert.NoError(t, allocatedGameServer.StartBackfill(ctx, assignment, 1))

	ticket4 := mustCreateTicket(t, frontend, &pb.Ticket{})
	{
		matches, err := director.FetchMatches(ctx, profile, mfConfig)
		assert.NoError(t, err)
		assert.Len(t, matches, 1)
		assert.Equal(t, false, matches[0].AllocateGameserver)
		assert.Len(t, matches[0].Tickets, 1)
		assert.NotNil(t, matches[0].Backfill)
		openSlots, err := getOpenSlots(matches[0].Backfill)
		assert.NoError(t, err)
		assert.Equal(t, int32(playersPerMatch-3), openSlots)

		res, err := director.AssignTickets(ctx, matches)
		assert.NoError(t, err)
		assert.Nil(t, res.AllocatedServer)
		assert.Len(t, res.AssignmentGroups, 0)

		assignment := mustAssignment(t, frontend, ticket4.Id, 3*time.Second)
		assert.Equal(t, string(allocatedGameServer.ConnectionName()), assignment.Connection)
		assert.NoError(t, allocatedGameServer.ConnectPlayer(ctx, ticket4.Id))
	}
}
