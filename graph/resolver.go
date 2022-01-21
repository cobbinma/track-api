//go:generate go run github.com/99designs/gqlgen
package graph

import (
	"github.com/ably/ably-go/ably"
	"github.com/cobbinma/track-api/repositories/postgres"
	"github.com/rs/zerolog/log"
	"os"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	queue      *ably.Realtime
	repository *postgres.Client
}

func NewResolver(repository *postgres.Client) *Resolver {
	key := os.Getenv("ABLY_API_KEY")
	if key == "" {
		log.Fatal().Str("key", "ABLY_API_KEY").Msg("key not given")
	}

	queue, err := ably.NewRealtime(ably.WithKey(key))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to create ably client")
	}

	return &Resolver{
		queue:      queue,
		repository: repository,
	}
}
