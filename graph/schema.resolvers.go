package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ably/ably-go/ably"
	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/cobbinma/track-api/graph/generated"
	"github.com/cobbinma/track-api/graph/model"
	"github.com/google/uuid"
)

func (r *mutationResolver) CreateJourney(ctx context.Context) (*model.Journey, error) {
	id := uuid.New()
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims")
	}

	journey := &model.Journey{
		ID:     id.String(),
		User:   &model.User{ID: claims.RegisteredClaims.Subject},
		Status: model.JourneyStatusActive,
	}

	if err := r.repository.CreateJourney(ctx, journey); err != nil {
		log.Printf("unable to create journey : %s", err)
		return nil, fmt.Errorf("unable to create journey")
	}

	return journey, nil
}

func (r *mutationResolver) UpdateJourneyStatus(ctx context.Context, input model.UpdateJourneyStatus) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}

	journey, err := r.repository.GetJourney(ctx, input.ID)
	if err != nil {
		log.Printf("unable to get journey : %s", err)
		return nil, fmt.Errorf("unable to get journey")
	}

	if user := claims.RegisteredClaims.Subject; journey.User.ID != user {
		log.Printf("unauthorized subject %q attempting to update journey %q", user, journey.ID)
		return nil, fmt.Errorf("unauthorized")
	}

	switch {
	case journey.Status == input.Status:
		break
	case journey.Status == model.JourneyStatusComplete:
		{
			log.Printf("journey %q is already complete\n", input.ID)
			break
		}
	case journey.Status == model.JourneyStatusActive, input.Status == model.JourneyStatusComplete:
		{
			journey.Status = input.Status
			journey.Position = nil

			if err := r.repository.UpdatePosition(ctx, journey.ID, journey.Position); err != nil {
				log.Printf("unable to update position : %s\n", err)
				return nil, fmt.Errorf("unable to update position")
			}

			if err := r.repository.UpdateStatus(ctx, journey.ID, journey.Status); err != nil {
				log.Printf("unable to update status : %s\n", err)
				return nil, fmt.Errorf("unable to update status")
			}

			message, err := json.Marshal(journey)
			if err != nil {
				log.Println("unable to marshal : ", err)
				return nil, err
			}
			if err := r.queue.Channels.Get(input.ID).
				Publish(ctx, "JourneyUpdate", string(message)); err != nil {
				log.Println("unable to publish : ", err)
				return nil, err
			}
		}
	default:
		log.Printf("unrecognised condition : journey status %q, input status %q\n",
			journey.Status, input.Status)
		return nil, fmt.Errorf("unrecognised state")
	}

	return journey, nil
}

func (r *mutationResolver) UpdateJourneyPosition(ctx context.Context, input model.UpdateJourneyPosition) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}

	journey, err := r.repository.GetJourney(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	if user := claims.RegisteredClaims.Subject; journey.User.ID != user {
		log.Printf("unauthorized subject %q attempting to update journey %q", user, journey.ID)
		return nil, fmt.Errorf("unauthorized")
	}

	if status := journey.Status; status != model.JourneyStatusActive {
		log.Printf("unable to update position for journey %q, status is %q", journey.ID, status)
		return nil, fmt.Errorf("unsupported update status")
	}

	log.Println("updating journey status")
	journey.Position = &model.Position{
		Lat: input.Position.Lat,
		Lng: input.Position.Lng,
	}

	if err := r.repository.UpdatePosition(ctx, journey.ID, journey.Position); err != nil {
		log.Printf("unable to update position : %s\n", err)
		return nil, fmt.Errorf("unable to update position")
	}

	channel := r.queue.Channels.Get(input.ID)
	message, err := json.Marshal(journey)
	if err != nil {
		log.Println("unable to marshal : ", err)
		return nil, err
	}
	if err := channel.Publish(ctx, "JourneyUpdate", string(message)); err != nil {
		log.Println("unable to publish : ", err)
		return nil, err
	}

	return journey, nil
}

func (r *subscriptionResolver) Journey(ctx context.Context, id string) (<-chan *model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}
	log.Printf("%q followed journey: %q\n", claims.RegisteredClaims.Subject, id)
	ch := make(chan *model.Journey, 1)

	journey, err := r.repository.GetJourney(ctx, id)
	if err != nil {
		log.Printf("unable to get journey : %s\n", err)
		return nil, fmt.Errorf("unable to get journey")
	}

	ch <- journey

	go func(ch chan *model.Journey) {
		unsubscribe, err := r.queue.Channels.Get(id).SubscribeAll(ctx, func(msg *ably.Message) {
			if data, ok := msg.Data.(string); ok {
				var j = &model.Journey{}
				if err := json.Unmarshal([]byte(data), j); err != nil {
					log.Println("unable to unmarshal : ", err)
					return
				}
				ch <- j
			} else {
				log.Println(fmt.Sprintf("unsupported message type: %T", msg.Data))
				return
			}
		})
		if err != nil {
			log.Println("unable to subscribe : ", err)
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
