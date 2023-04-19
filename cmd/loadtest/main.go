package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/castaneai/omtools"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	flag.Float64Var(&rps, "rps", 1.0, "RPS (request per second)")
	flag.StringVar(&frontendAddr, "frontend", "localhost:50504", "An address of Open Match frontend")
	flag.StringVar(&backendAddr, "backend", "localhost:50505", "An address of Open Match backend")
	flag.Parse()

	log.Printf("open match load-testing (rps: %.2f, frontend addr: %s)", rps, frontendAddr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	omFrontend, err := newOMFrontendClient(frontendAddr)
	if err != nil {
		log.Fatalf("failed to new open-match frontend client: %+v", err)
	}
	omBackend, err := newOMBackendClient(backendAddr)
	if err != nil {
		log.Fatalf("failed to new open-match frontend client: %+v", err)
	}
	director := omtools.NewDirector(omBackend, matchProfile, &pb.FunctionConfig{
		Host: "matchfunction.open-match.svc.cluster.local.",
		Port: 50502,
		Type: pb.FunctionConfig_GRPC,
	}, AssignFunc(assign))
	directorErr := make(chan error)
	go func() {
		directorErr <- director.Run(ctx, 500*time.Millisecond)
	}()

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

type AssignFunc func(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)

func (f AssignFunc) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	return f(ctx, matches)
}

func assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  ticketIDs(match),
			Assignment: &pb.Assignment{Connection: "dummy"},
		})
	}
	return asgs, nil
}

func ticketIDs(match *pb.Match) []string {
	var ids []string
	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
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

func newOMFrontendClient(addr string) (pb.FrontendServiceClient, error) {
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	cc, err := grpc.Dial(addr, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to open match frontend: %w", err)
	}
	return pb.NewFrontendServiceClient(cc), nil
}

func newOMBackendClient(addr string) (pb.BackendServiceClient, error) {
	opts := grpc.WithTransportCredentials(insecure.NewCredentials())
	cc, err := grpc.Dial(addr, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to open match backend: %w", err)
	}
	return pb.NewBackendServiceClient(cc), nil
}
