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
	"github.com/rs/zerolog/log"
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
		log.Fatal().Err(err).Msg("failed to parse the issuer url")
	}

	// Set up the validator.
	jwtValidator, err := validator.New(
		jwks.NewCachingProvider(issuerURL, 5*time.Minute).KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{os.Getenv("AUTH0_AUDIENCE")},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to set up the validator")
	}

	srv.AddTransport(transport.POST{})
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				match := r.Header.Get(echo.HeaderOrigin) == origin
				if !match {
					log.Warn().Str("origin", r.Header.Get(echo.HeaderOrigin)).
						Str("expected", origin).Msg("websocket origin does not match expected")
				}
				return match
			},
			ReadBufferSize:   1024,
			WriteBufferSize:  1024,
			HandshakeTimeout: 5 * time.Second,
		},
		KeepAlivePingInterval: 10 * time.Second,
		PingPongInterval:      time.Second,
		InitFunc: func(ctx context.Context, p transport.InitPayload) (context.Context, error) {
			token, err := jwtValidator.ValidateToken(ctx, strings.TrimPrefix(p.Authorization(), "Bearer "))
			if err != nil {
				log.Warn().Err(err).Msg("unable to initialise websocket connection")
				return nil, ErrUnAuthorized
			}

			claims, ok := token.(*validator.ValidatedClaims)
			if !ok {
				log.Warn().Msg("unexpected token format")
				return nil, ErrUnAuthorized
			}

			return context.WithValue(ctx, jwtmiddleware.ContextKey{}, claims), nil
		},
	})

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
