package main

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/cobbinma/track-api/graph"
	"github.com/cobbinma/track-api/graph/generated"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"os"
)

const defaultPort = "8080"

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	e := graph.NewRouter(echo.New(), handler.New(
		generated.NewExecutableSchema(generated.Config{Resolvers: graph.NewResolver()})))
	e.Logger.Fatal(e.Start(":" + port))
}
