package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/castaneai/openmatch-local-dev/omutils"
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
	var builtinDirector bool
	flag.Float64Var(&rps, "rps", 1.0, "RPS (request per second)")
	flag.StringVar(&frontendAddr, "frontend", "localhost:50504", "An address of Open Match frontend")
	flag.StringVar(&backendAddr, "backend", "localhost:50505", "An address of Open Match backend")
	flag.BoolVar(&builtinDirector, "builtin-director", true, "Enabling built-in director")
	flag.Parse()

	log.Printf("open match load-testing (rps: %.2f, frontend addr: %s)", rps, frontendAddr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if builtinDirector {
		director, err := omutils.NewTestDirector(backendAddr, matchProfile)
		if err != nil {
			log.Fatalf("failed to new test director: %+v", err)
		}
		go func() {
			if err := director.Run(ctx, 2*time.Second); err != nil {
				log.Printf("failed to run director: %+v", err)
			}
		}()
	}

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
		}
	}
}

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
	log.Printf("ticket %s assigned to %s", ticket.Id, resp.Assignment.Connection)
}
