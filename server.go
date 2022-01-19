package main

import (
	"context"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/cobbinma/track-api/graph"
	"github.com/cobbinma/track-api/graph/generated"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"os"
)

const defaultPort = "8080"

func main() {
	_ = godotenv.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().
		ApplyURI(os.Getenv("MONGO_DB_URL")).
		SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1)))
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	e := graph.NewRouter(echo.New(), handler.New(
		generated.NewExecutableSchema(generated.Config{Resolvers: graph.NewResolver(client)})))
	e.Logger.Fatal(e.Start(":" + port))
}
