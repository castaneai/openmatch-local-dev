package tests

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
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
	players        map[string]struct{}
	mu             sync.RWMutex
	logger         *log.Logger
	backfillWorker atomic.Pointer[backfillWorker]
}

func AllocateGameServer(omFrontend pb.FrontendServiceClient) *GameServer {
	gameServerMapMu.Lock()
	defer gameServerMapMu.Unlock()
	connName := GameServerConnectionName(uuid.Must(uuid.NewRandom()).String())
	logger := log.New(os.Stderr, fmt.Sprintf("[GS: %s] ", connName), log.LstdFlags)
	gameServerMap[connName] = &GameServer{
		omFrontend:     omFrontend,
		connectionName: connName,
		players:        map[string]struct{}{},
		mu:             sync.RWMutex{},
		logger:         logger,
	}
	logger.Printf("allocated")
	return gameServerMap[connName]
}

func (gs *GameServer) ConnectionName() GameServerConnectionName {
	return gs.connectionName
}

func (gs *GameServer) ConnectPlayer(ctx context.Context, ticketID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.players[ticketID]; exists {
		gs.log("player re-connected (ticketID: %s) (%d players in room)", ticketID, len(gs.players))
		return nil
	}

	newPlayerCount := len(gs.players) + 1
	if newPlayerCount > playersPerMatch {
		return ErrGameServerCapacityExceeded
	}
	gs.players[ticketID] = struct{}{}
	gs.log("player connected (ticketID: %s) (%d players in room)", ticketID, newPlayerCount)
	return nil
}

func (gs *GameServer) DisconnectPlayer(ctx context.Context, ticketID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if _, exists := gs.players[ticketID]; !exists {
		return nil
	}
	delete(gs.players, ticketID)

	newPlayerCount := len(gs.players)
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
	gs.backfillWorker.Store(startBackfillWorker(gs.omFrontend, backfill, assignment))
	gs.log("start polling with acknowledge backfill")
}

func (gs *GameServer) StopBackfill() error {
	if w := gs.backfillWorker.Load(); w != nil {
		w.Stop()
	}
	return nil
}

func (gs *GameServer) log(format string, args ...interface{}) {
	gs.logger.Printf(format, args...)
}

type backfillWorker struct {
	backfill   *pb.Backfill
	omFrontend pb.FrontendServiceClient
	stop       context.CancelFunc
}

func startBackfillWorker(omFrontend pb.FrontendServiceClient, backfill *pb.Backfill, assignment *pb.Assignment) *backfillWorker {
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(acknowledgeBackfillInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := omFrontend.AcknowledgeBackfill(ctx, &pb.AcknowledgeBackfillRequest{
					BackfillId: backfill.Id,
					Assignment: assignment,
				}); err != nil {
					if ctx.Err() != nil {
						return
					}
					continue
				}
			}
		}
	}()
	return &backfillWorker{
		backfill:   backfill,
		omFrontend: omFrontend,
		stop:       stop,
	}
}

func (b *backfillWorker) Stop() {
	b.stop()
	_, _ = b.omFrontend.DeleteBackfill(context.Background(), &pb.DeleteBackfillRequest{
		BackfillId: b.backfill.Id,
	})
}
