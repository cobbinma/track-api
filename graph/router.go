package graph

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	jwtmiddleware "github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func NewRouter(e *echo.Echo, srv *handler.Server) *echo.Echo {
	origin := os.Getenv("ORIGIN")
	issuerURL, err := url.Parse(fmt.Sprintf("https://%s/", os.Getenv("AUTH0_DOMAIN")))
	if err != nil {
		log.Fatalf("failed to parse the issuer url: %v", err)
	}

	// Set up the validator.
	jwtValidator, err := validator.New(
		jwks.NewCachingProvider(issuerURL, 5*time.Minute).KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{os.Getenv("AUTH0_AUDIENCE")},
	)
	if err != nil {
		log.Fatalf("failed to set up the validator: %v", err)
	}

	// Configure WebSocket with CORS
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return r.Header.Get(echo.HeaderOrigin) == origin
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		KeepAlivePingInterval: 10 * time.Second,
		InitFunc: func(ctx context.Context, p transport.InitPayload) (context.Context, error) {
			token, err := jwtValidator.ValidateToken(ctx, strings.TrimPrefix(p.Authorization(), "Bearer "))
			if err != nil {
				log.Println("unable to initialise websocket connection : ", err)
				return nil, err
			}

			claims, ok := token.(*validator.ValidatedClaims)
			if !ok {
				log.Println("unexpected token format")
				return nil, err
			}

			return context.WithValue(ctx, jwtmiddleware.ContextKey{}, claims), nil
		},
	})
	srv.AddTransport(transport.POST{})

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK!")
	})
	e.GET("/playground", func(c echo.Context) error {
		playground.Handler("GraphQL", "/query").ServeHTTP(c.Response(), c.Request())
		return nil
	})

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{origin},
	}))

	e.POST("/query", func(c echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	}, echo.WrapMiddleware(jwtmiddleware.New(jwtValidator.ValidateToken).CheckJWT))

	e.GET("/subscriptions", func(c echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	return e
}
