//go:generate go run github.com/99designs/gqlgen
package graph

import (
	"github.com/ably/ably-go/ably"
	"github.com/cobbinma/track-api/graph/model"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"os"
	"sync"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	queue *ably.Realtime
	mu    sync.RWMutex // nolint: structcheck
	rooms map[string]*Room
	mongo *mongo.Client
}

type Room struct {
	journey *model.Journey
}

func NewResolver(mongo *mongo.Client) *Resolver {
	key := os.Getenv("ABLY_API_KEY")
	if key == "" {
		log.Fatalf("key not given")
	}
	queue, err := ably.NewRealtime(ably.WithKey(key))
	if err != nil {
		log.Fatalf("unable to create ably client : %s", err)
	}

	return &Resolver{
		queue: queue,
		rooms: map[string]*Room{},
		mongo: mongo,
	}
}
