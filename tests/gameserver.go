package tests

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"open-match.dev/open-match/pkg/pb"
)

const (
	acknowledgeBackfillInterval = 500 * time.Millisecond
)

type GameServerConnectionName string

var gameServerMap = map[GameServerConnectionName]*GameServer{}
var gameServerMapMu sync.RWMutex

type GameServer struct {
	omFrontend     pb.FrontendServiceClient
	connectionName GameServerConnectionName
	playerTickets  map[string]struct{}
	mu             sync.RWMutex
	logger         *log.Logger
	stopBackfill   context.CancelFunc
}

func AllocateGameServer(connName GameServerConnectionName, omFrontend pb.FrontendServiceClient) *GameServer {
	gameServerMapMu.Lock()
	defer gameServerMapMu.Unlock()
	gameServerMap[connName] = &GameServer{
		omFrontend:     omFrontend,
		connectionName: connName,
		playerTickets:  map[string]struct{}{},
		mu:             sync.RWMutex{},
		logger:         log.New(os.Stderr, fmt.Sprintf("[GS: %s] ", connName), log.LstdFlags),
	}
	return gameServerMap[connName]
}

func (gs *GameServer) ConnectionName() GameServerConnectionName {
	return gs.connectionName
}

func (gs *GameServer) Connect(ticketID string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.playerTickets[ticketID] = struct{}{}
	playerCount := len(gs.playerTickets)
	gs.log("player connected (ticketID: %s) (%d players in room)", ticketID, playerCount)
}

func (gs *GameServer) Disconnect(ticketID string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	delete(gs.playerTickets, ticketID)
	playerCount := len(gs.playerTickets)
	gs.log("player disconnected (ticketID: %s) (%d players in room)", ticketID, playerCount)
}

func (gs *GameServer) StartBackfill(ctx context.Context, assignment *pb.Assignment, openSlots int) error {
	req := &pb.Backfill{}
	if err := setOpenSlots(req, int32(openSlots)); err != nil {
		return err
	}
	backfill, err := gs.omFrontend.CreateBackfill(ctx, &pb.CreateBackfillRequest{Backfill: req})
	if err != nil {
		return err
	}
	gs.log("backfill created (backfillID: %s, openSlots: %d)", backfill.Id, openSlots)
	gs.StartBackfillCreated(backfill, assignment)
	return nil
}

func (gs *GameServer) StartBackfillCreated(backfill *pb.Backfill, assignment *pb.Assignment) {
	// The allocated GameServer starts polling Open Match to acknowledge the backfill
	// ref: https://open-match.dev/site/docs/guides/backfill/
	pollingCtx, cancel := context.WithCancel(context.Background())
	gs.stopBackfill = cancel
	go func() {
		ticker := time.NewTicker(acknowledgeBackfillInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pollingCtx.Done():
				return
			case <-ticker.C:
				if _, err := gs.omFrontend.AcknowledgeBackfill(pollingCtx, &pb.AcknowledgeBackfillRequest{
					BackfillId: backfill.Id,
					Assignment: assignment,
				}); err != nil {
					gs.log("failed to acknowledge backfill: %+v", err)
				}
			}
		}
	}()
	gs.log("start polling with acknowledge backfill")
}

func (gs *GameServer) StopBackfill(ctx context.Context, backfillID string) error {
	if gs.stopBackfill != nil {
		gs.stopBackfill()
	}
	if _, err := gs.omFrontend.DeleteBackfill(ctx, &pb.DeleteBackfillRequest{BackfillId: backfillID}); err != nil {
		return err
	}
	gs.log("backfill stopped (backfillID: %s)", backfillID)
	return nil
}

func (gs *GameServer) log(format string, args ...interface{}) {
	gs.logger.Printf(format, args...)
}
