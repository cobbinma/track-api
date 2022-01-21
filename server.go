package main

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/cobbinma/track-api/graph"
	"github.com/cobbinma/track-api/graph/generated"
	"github.com/cobbinma/track-api/repositories/postgres"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"net/url"
	"os"
)

const defaultPort = "8080"

func main() {
	_ = godotenv.Load()

	dbu, err := url.Parse(os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	pg, err := postgres.NewPostgres(*dbu, "file://repositories/postgres/migrations")
	if err != nil {
		panic(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	e := graph.NewRouter(echo.New(), handler.New(
		generated.NewExecutableSchema(generated.Config{Resolvers: graph.NewResolver(pg)})))
	e.Logger.Fatal(e.Start(":" + port))
}
