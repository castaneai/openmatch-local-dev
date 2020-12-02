package main

import (
	"context"
	"log"

	"github.com/davecgh/go-spew/spew"

	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"
)

func main() {
	// See portForward section in skaffold.yaml
	omFrontendAddr := "localhost:50504"
	omFrontend, err := newOMFrontendClient(omFrontendAddr)
	if err != nil {
		log.Fatalf("failed to connect to open-match frontend: %+v", err)
	}

	ctx := context.Background()
	ticket, err := omFrontend.CreateTicket(ctx, &pb.CreateTicketRequest{Ticket: &pb.Ticket{
		SearchFields: &pb.SearchFields{Tags: []string{"aaaaa"}},
	}})
	if err != nil {
		log.Fatalf("failed to create ticket: %+v", err)
	}
	spew.Printf("ticket created: %+v", ticket)
}

func newOMFrontendClient(addr string) (pb.FrontendServiceClient, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return pb.NewFrontendServiceClient(cc), nil
}
