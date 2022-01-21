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
	"go.mongodb.org/mongo-driver/bson"
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

	if _, err := r.mongo.Database("track_db").
		Collection("journeys").InsertOne(ctx, journey); err != nil {
		log.Printf("unable to insert journey : %s", err)
		return nil, fmt.Errorf("unable to insert journey")
	}

	return journey, nil
}

func (r *mutationResolver) UpdateJourneyStatus(ctx context.Context, input model.UpdateJourneyStatus) (*model.Journey, error) {
	claims, ok := ctx.Value(jwtmiddleware.ContextKey{}).(*validator.ValidatedClaims)
	if !ok {
		log.Println("no claims in context")
		return nil, fmt.Errorf("no claims in context")
	}

	var journey = &model.Journey{}
	collection := r.mongo.Database("track_db").Collection("journeys")
	if err := collection.FindOne(ctx, bson.D{{"id", input.ID}}).Decode(journey); err != nil {
		log.Printf("unable to find journey : %s", err)
		return nil, fmt.Errorf("journey not found")
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

			if _, err := collection.InsertOne(ctx, journey); err != nil {
				log.Printf("unable to insert journey : %s", err)
				return nil, fmt.Errorf("unable to insert journey")
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

	var journey = &model.Journey{}
	collection := r.mongo.Database("track_db").Collection("journeys")
	if err := collection.FindOne(ctx, bson.D{{"id", input.ID}}).Decode(journey); err != nil {
		log.Printf("unable to find journey : %s", err)
		return nil, fmt.Errorf("journey not found")
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

	if _, err := collection.InsertOne(ctx, journey); err != nil {
		log.Println("unable to insert journey in mongodb : ", err)
		return nil, fmt.Errorf("unable to insert journey")
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

	var journey = &model.Journey{}
	if err := r.mongo.Database("track_db").Collection("journeys").
		FindOne(ctx, bson.D{{"id", id}}).Decode(journey); err != nil {
		log.Println("unable to find journey : ", err)
		return nil, fmt.Errorf("journey not found")
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
