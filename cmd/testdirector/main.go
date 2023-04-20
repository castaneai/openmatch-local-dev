package main

import (
	"context"
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
}

func main() {
	backendAddr := "open-match-backend.open-match.svc.cluster.local.:50505"
	log.Printf("start testdirector (backend: %s, profile: %s)", backendAddr, matchProfile.Name)
	d, err := omutils.NewTestDirector(backendAddr, matchProfile)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := d.Run(ctx, 1*time.Second); err != nil {
		log.Fatal(err)
	}
}
