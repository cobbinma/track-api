//go:generate go run github.com/99designs/gqlgen
package graph

import (
	"github.com/ably/ably-go/ably"
	"github.com/cobbinma/track-api/graph/model"
	"github.com/cobbinma/track-api/repositories/postgres"
	"log"
	"os"
	"sync"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	queue      *ably.Realtime
	mu         sync.RWMutex // nolint: structcheck
	rooms      map[string]*Room
	repository *postgres.Client
}

type Room struct {
	journey *model.Journey
}

func NewResolver(repository *postgres.Client) *Resolver {
	key := os.Getenv("ABLY_API_KEY")
	if key == "" {
		log.Fatalf("key not given")
	}

	queue, err := ably.NewRealtime(ably.WithKey(key))
	if err != nil {
		log.Fatalf("unable to create ably client : %s", err)
	}

	return &Resolver{
		queue:      queue,
		rooms:      map[string]*Room{},
		repository: repository,
	}
}
