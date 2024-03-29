package tests

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"open-match.dev/open-match/pkg/pb"
)

const (
	acknowledgeBackfillInterval = 10 * time.Millisecond
)

var (
	ErrGameServerCapacityExceeded = errors.New("gameserver capacity exceeded")
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
	backfillID     string
	stopBackfill   context.CancelFunc
}

func AllocateGameServer(omFrontend pb.FrontendServiceClient) *GameServer {
	gameServerMapMu.Lock()
	defer gameServerMapMu.Unlock()
	connName := GameServerConnectionName(uuid.Must(uuid.NewRandom()).String())
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

func (gs *GameServer) ConnectPlayer(ctx context.Context, ticketID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.playerTickets[ticketID]; exists {
		gs.log("player re-connected (ticketID: %s) (%d players in room)", ticketID, len(gs.playerTickets))
		return nil
	}

	newPlayerCount := len(gs.playerTickets) + 1
	if newPlayerCount > playersPerMatch {
		return ErrGameServerCapacityExceeded
	}
	gs.playerTickets[ticketID] = struct{}{}
	gs.log("player connected (ticketID: %s) (%d players in room)", ticketID, newPlayerCount)
	return nil
}

func (gs *GameServer) DisconnectPlayer(ctx context.Context, ticketID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.playerTickets[ticketID]; !exists {
		return nil
	}
	delete(gs.playerTickets, ticketID)

	newPlayerCount := len(gs.playerTickets)
	gs.log("player disconnected (ticketID: %s) (%d players in room)", ticketID, newPlayerCount)
	return nil
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
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.stopBackfill = cancel
	go func() {
		ticker := time.NewTicker(acknowledgeBackfillInterval)
		defer ticker.Stop()
		openSlots := int32(0)
		for {
			select {
			case <-pollingCtx.Done():
				return
			case <-ticker.C:
				bf, err := gs.omFrontend.AcknowledgeBackfill(pollingCtx, &pb.AcknowledgeBackfillRequest{
					BackfillId: backfill.Id,
					Assignment: assignment,
				})
				if err != nil {
					if pollingCtx.Err() != nil {
						return
					}
					gs.log("failed to acknowledge backfill: %+v", err)
					continue
				}
				slots, err := getOpenSlots(bf)
				if err != nil {
					gs.log("failed to get openSlots: %+v", err)
					continue
				}
				if slots != openSlots {
					gs.log("acknowledge backfill (openSlots: %d)", slots)
					openSlots = slots
				}
			}
		}
	}()
	gs.backfillID = backfill.Id
	gs.log("start polling with acknowledge backfill")
}

func (gs *GameServer) StopBackfill(ctx context.Context, backfillID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.stopBackfill != nil {
		gs.stopBackfill()
	}
	gs.backfillID = ""
	if _, err := gs.omFrontend.DeleteBackfill(ctx, &pb.DeleteBackfillRequest{BackfillId: backfillID}); err != nil {
		return err
	}
	gs.log("backfill stopped (backfillID: %s)", backfillID)
	return nil
}

func (gs *GameServer) log(format string, args ...interface{}) {
	gs.logger.Printf(format, args...)
}
