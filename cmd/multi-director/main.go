package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/castaneai/openmatch-local-dev/omutils"
	"golang.org/x/sync/errgroup"
	"open-match.dev/open-match/pkg/pb"
)

var matchProfile = &pb.MatchProfile{
	Name: "test-profile",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
	Extensions: nil,
}

func main() {
	var rps float64
	var frontendAddr, backendAddr string
	flag.Float64Var(&rps, "rps", 3.0, "RPS (request per second)")
	flag.StringVar(&frontendAddr, "frontend", "localhost:50504", "An address of Open Match frontend")
	flag.StringVar(&backendAddr, "backend", "localhost:50505", "An address of Open Match backend")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	eg, ctx := errgroup.WithContext(ctx)
	directorCount := 10
	for i := 0; i < directorCount; i++ {
		eg.Go(func() error {
			director, err := omutils.NewTestDirector(backendAddr, matchProfile)
			if err != nil {
				log.Fatalf("failed to new test director: %+v", err)
			}
			return director.Run(ctx, 500*time.Millisecond)
		})
	}
	directorErr := make(chan error)
	go func() { directorErr <- eg.Wait() }()

	omFrontend, err := omutils.NewOMFrontendClient(frontendAddr)
	if err != nil {
		log.Fatalf("failed to new om frontend client: %+v", err)
	}
	tick := time.Duration(1.0 / rps * float64(time.Second))
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticket, err := omFrontend.CreateTicket(ctx, &pb.CreateTicketRequest{
				Ticket: &pb.Ticket{},
			})
			if err != nil {
				log.Printf("failed to create ticket: %+v", err)
				continue
			}
			log.Printf("ticket created: %s", ticket.Id)
			go watchTickets(ctx, omFrontend, ticket)
		case err := <-directorErr:
			log.Printf("director stopped with error: %+v", err)
			return
		}
	}
}

var mu sync.RWMutex
var assignedTickets = map[string]string{}

func watchTickets(ctx context.Context, omFrontend pb.FrontendServiceClient, ticket *pb.Ticket) {
	stream, err := omFrontend.WatchAssignments(ctx, &pb.WatchAssignmentsRequest{TicketId: ticket.Id})
	if err != nil {
		return
	}
	resp, err := stream.Recv()
	if errors.Is(err, io.EOF) {
		return
	}
	if err != nil {
		log.Printf("failed to recv watch assignments: %+v", err)
		return
	}

	mu.RLock()
	conn, ok := assignedTickets[ticket.Id]
	mu.RUnlock()
	if ok && resp.Assignment.Connection == conn {
		log.Printf("assignment overlapped!!")
		return
	}

	mu.Lock()
	assignedTickets[ticket.Id] = resp.Assignment.Connection
	mu.Unlock()
	log.Printf("ticket(%s) asssigned to %s", ticket.Id, resp.Assignment.Connection)
}
