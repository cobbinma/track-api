package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ably/ably-go/ably"
	"github.com/cobbinma/track-api/graph/generated"
	"github.com/cobbinma/track-api/graph/model"
	"github.com/google/uuid"
	"log"
)

func (r *mutationResolver) CreateJourney(_ context.Context, input model.NewJourney) (*model.Journey, error) {
	id := uuid.New()
	journey := &model.Journey{
		ID:     id.String(),
		User:   &model.User{ID: input.UserID},
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
	r.mu.RLock()
	room, ok := r.rooms[input.ID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("journey not found")
	}

	if room.journey.Status != input.Status {
		log.Println("updating journey status")
		r.mu.Lock()
		room.journey.Status = input.Status
		r.mu.Unlock()
		channel := r.queue.Channels.Get(input.ID)
		message, err := json.Marshal(room.journey)
		if err != nil {
			log.Println("ERROR: ", err)
			return nil, err
		}
		if err := channel.Publish(ctx, "JourneyUpdate", string(message)); err != nil {
			panic(err)
		}
	}

	return room.journey, nil
}

func (r *subscriptionResolver) Journey(ctx context.Context, id string) (<-chan *model.Journey, error) {
	log.Println("subscription to journey: ", id)
	ch := make(chan *model.Journey, 1)

	r.mu.RLock()
	room, ok := r.rooms[id]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("journey not found")
	}
	ch <- room.journey

	go func(ch chan *model.Journey) {
		log.Println("follower joined")
		defer log.Println("follower left")
		unsubscribe, err := r.queue.Channels.Get(id).SubscribeAll(ctx, func(msg *ably.Message) {
			if data, ok := msg.Data.(string); ok {
				var journey *model.Journey
				if err := json.Unmarshal([]byte(data), &journey); err != nil {
					log.Println("ERROR: ", err)
					return
				}
				ch <- journey
			} else {
				log.Println(fmt.Sprintf("unsupported message type: %T", msg.Data))
				return
			}
		})
		if err != nil {
			log.Println("ERROR: ", err)
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
