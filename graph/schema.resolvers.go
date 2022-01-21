package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ably/ably-go/ably"
	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/cobbinma/track-api/graph/generated"
	"github.com/cobbinma/track-api/graph/model"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	ErrUnAuthorized = fmt.Errorf("unauthorized")
	ErrUnexpected   = fmt.Errorf("unexpected error")
	ErrBadRequest   = fmt.Errorf("bad request")
)

func (r *mutationResolver) CreateJourney(ctx context.Context) (*model.Journey, error) {
	id := uuid.New()
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Warn().Msg("no claims in context")
		return nil, ErrUnAuthorized
	}

	journey := &model.Journey{
		ID:     id.String(),
		User:   &model.User{ID: claims.RegisteredClaims.Subject},
		Status: model.JourneyStatusActive,
	}

	if err := r.repository.CreateJourney(ctx, journey); err != nil {
		log.Error().Err(err).Msg("unable to create journey in repository")
		return nil, ErrUnexpected
	}

	return journey, nil
}

func (r *mutationResolver) UpdateJourneyStatus(ctx context.Context, input model.UpdateJourneyStatus) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Warn().Msg("no claims in context")
		return nil, ErrUnAuthorized
	}

	journey, err := r.repository.GetJourney(ctx, input.ID)
	if err != nil {
		log.Error().Err(err).Msg("unable to get journey from repository")
		return nil, ErrUnexpected
	}

	if user := claims.RegisteredClaims.Subject; journey.User.ID != user {
		log.Warn().Str("subject", user).Str("journeyId", journey.ID).
			Msg("unauthorized subject attempting to update journey")
		return nil, ErrUnAuthorized
	}

	switch {
	case journey.Status == input.Status:
		break
	case journey.Status == model.JourneyStatusComplete:
		{
			log.Warn().Str("journeyId", input.ID).Msg("journey is already complete")
			break
		}
	case journey.Status == model.JourneyStatusActive, input.Status == model.JourneyStatusComplete:
		{
			journey.Status = input.Status
			journey.Position = nil

			if err := r.repository.UpdatePosition(ctx, journey.ID, journey.Position); err != nil {
				log.Error().Err(err).Msg("unable to update position in repository")
				return nil, ErrUnexpected
			}

			if err := r.repository.UpdateStatus(ctx, journey.ID, journey.Status); err != nil {
				log.Error().Err(err).Msg("unable to update status in repository")
				return nil, ErrUnexpected
			}

			message, err := json.Marshal(journey)
			if err != nil {
				log.Error().Err(err).Msg("unable to marshal message")
				return nil, ErrUnexpected
			}

			if err := r.queue.Channels.Get(input.ID).
				Publish(ctx, "JourneyUpdate", string(message)); err != nil {
				log.Error().Err(err).Str("journeyId", input.ID).Msg("unable to publish message in queue")
				return nil, ErrUnexpected
			}
		}
	default:
		log.Error().Str("current_status", journey.Status.String()).
			Str("input_status", input.Status.String()).Msg("unrecognised condition")
		return nil, ErrUnexpected
	}

	return journey, nil
}

func (r *mutationResolver) UpdateJourneyPosition(ctx context.Context, input model.UpdateJourneyPosition) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Warn().Msg("no claims in context")
		return nil, ErrUnAuthorized
	}

	journey, err := r.repository.GetJourney(ctx, input.ID)
	if err != nil {
		log.Error().Err(err).Msg("unable to get journey from repository")
		return nil, ErrUnexpected
	}

	if user := claims.RegisteredClaims.Subject; journey.User.ID != user {
		log.Warn().Str("subject", user).Str("journeyId", journey.ID).
			Msg("unauthorized subject attempting to update journey")
	}

	if status := journey.Status; status != model.JourneyStatusActive {
		log.Warn().Str("journeyId", journey.ID).Str("status", status.String()).
			Msg("unsupported update position status")
		return nil, ErrBadRequest
	}

	journey.Position = &model.Position{
		Lat: input.Position.Lat,
		Lng: input.Position.Lng,
	}

	if err := r.repository.UpdatePosition(ctx, journey.ID, journey.Position); err != nil {
		log.Error().Err(err).Msg("unable to update position in repository")
		return nil, ErrUnexpected
	}

	message, err := json.Marshal(journey)
	if err != nil {
		log.Error().Err(err).Msg("unable to marshal message")
		return nil, ErrUnexpected
	}

	if err := r.queue.Channels.Get(input.ID).Publish(ctx, "JourneyUpdate", string(message)); err != nil {
		log.Error().Err(err).Str("journeyId", input.ID).Msg("unable to publish message in queue")
		return nil, ErrUnexpected
	}

	return journey, nil
}

func (r *subscriptionResolver) Journey(ctx context.Context, id string) (<-chan *model.Journey, error) {
	if _, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims); !ok {
		log.Warn().Msg("no claims in context")
		return nil, ErrUnAuthorized
	}

	ch := make(chan *model.Journey, 1)

	journey, err := r.repository.GetJourney(ctx, id)
	if err != nil {
		log.Error().Err(err).Msg("unable to get journey from repository")
		return nil, ErrUnexpected
	}

	ch <- journey

	go func(ch chan *model.Journey) {
		unsubscribe, err := r.queue.Channels.Get(id).SubscribeAll(ctx, func(msg *ably.Message) {
			if data, ok := msg.Data.(string); ok {
				var j = &model.Journey{}
				if err := json.Unmarshal([]byte(data), j); err != nil {
					log.Error().Err(err).Msg("unable to unmarshal message")
					return
				}
				ch <- j
			} else {
				log.Error().Err(err).Msgf("unsupported message type: %T", msg.Data)
				return
			}
		})
		if err != nil {
			log.Error().Err(err).Msgf("unable to subscribe")
			return
		}

		<-ctx.Done()
		unsubscribe()
	}(ch)

	return ch, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
