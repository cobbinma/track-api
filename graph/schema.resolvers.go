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
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms[id.String()] = &Room{
		journey: journey,
	}

	return journey, nil
}

func (r *mutationResolver) UpdateJourneyStatus(ctx context.Context, input model.UpdateJourneyStatus) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}

	r.mu.RLock()
	room, ok := r.rooms[input.ID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("journey not found")
	}

	if user := claims.RegisteredClaims.Subject; room.journey.User.ID != user {
		log.Printf("unauthorized subject %q attempting to update journey %q", user, room.journey.ID)
		return nil, fmt.Errorf("unauthorized")
	}

	if room.journey.Status != input.Status {
		log.Println("updating journey status")
		r.mu.Lock()
		room.journey.Status = input.Status
		if input.Status == model.JourneyStatusComplete {
			room.journey.Position = nil
		}
		r.mu.Unlock()
		channel := r.queue.Channels.Get(input.ID)
		message, err := json.Marshal(room.journey)
		if err != nil {
			log.Println("unable to marshal : ", err)
			return nil, err
		}
		if err := channel.Publish(ctx, "JourneyUpdate", string(message)); err != nil {
			log.Println("unable to publish : ", err)
			return nil, err
		}
	}

	return room.journey, nil
}

func (r *mutationResolver) UpdateJourneyPosition(ctx context.Context, input model.UpdateJourneyPosition) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}

	r.mu.RLock()
	room, ok := r.rooms[input.ID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("journey not found")
	}

	if user := claims.RegisteredClaims.Subject; room.journey.User.ID != user {
		log.Printf("unauthorized subject %q attempting to update journey %q", user, room.journey.ID)
		return nil, fmt.Errorf("unauthorized")
	}

	if status := room.journey.Status; status != model.JourneyStatusActive {
		log.Printf("unable to update position for journey %q, status is %q", room.journey.ID, status)
		return nil, fmt.Errorf("unsupported update status")
	}

	log.Println("updating journey status")
	r.mu.Lock()
	room.journey.Position = &model.Position{
		Lat: input.Position.Lat,
		Lng: input.Position.Lng,
	}
	r.mu.Unlock()
	channel := r.queue.Channels.Get(input.ID)
	message, err := json.Marshal(room.journey)
	if err != nil {
		log.Println("unable to marshal : ", err)
		return nil, err
	}
	if err := channel.Publish(ctx, "JourneyUpdate", string(message)); err != nil {
		log.Println("unable to publish : ", err)
		return nil, err
	}

	return room.journey, nil
}

func (r *subscriptionResolver) Journey(ctx context.Context, id string) (<-chan *model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}
	log.Printf("%q followed journey: %q\n", claims.RegisteredClaims.Subject, id)
	ch := make(chan *model.Journey, 1)

	r.mu.RLock()
	room, ok := r.rooms[id]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("journey not found")
	}
	ch <- room.journey

	go func(ch chan *model.Journey) {
		unsubscribe, err := r.queue.Channels.Get(id).SubscribeAll(ctx, func(msg *ably.Message) {
			if data, ok := msg.Data.(string); ok {
				var journey *model.Journey
				if err := json.Unmarshal([]byte(data), &journey); err != nil {
					log.Println("unable to unmarshal : ", err)
					return
				}
				ch <- journey
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
