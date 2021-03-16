// Package server is responsible for the HTTP server that serves fediverse.express
package server

import (
	"log"
	"strings"

	"github.com/CuteAP/fediverse.express/server/endpoints/steps"
	"github.com/CuteAP/fediverse.express/server/srvcommon"
	"github.com/CuteAP/fediverse.express/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

type Server struct {
	store       *session.Store
	providers   map[string]srvcommon.Provider
	app         *fiber.App
	stepHandler *steps.Steps
}

// registerMiddlewares registers supported middlewares to the
// HTTP server and must be called before Listen
func (s *Server) registerMiddlewares() {
	s.app.Use(func(ctx *fiber.Ctx) error {
		session, err := s.store.Get(ctx)
		if err != nil {
			log.Fatalf("Could not create session: %v", err)
		}

		ctx.Locals("session", session)
		ctx.Next()
		return nil
	})
	s.app.Use(func(ctx *fiber.Ctx) error {
		if strings.HasPrefix(string(ctx.Context().Path()), "/step/") {
			session := ctx.Locals("session").(*session.Session)

			if session.Get("provider") == nil || session.Get("accessToken") == nil || session.Get("privateKey") == nil {
				log.Printf("Could not determine provider, accessToken, or privateKey: provider=%s, accessToken nil? %t, privateKey nil? %t", session.Get("provider"), session.Get("accessToken") == nil, session.Get("privateKey") == nil)
				ctx.Redirect("/")
				return nil
			}
		}

		ctx.Next()
		return nil
	})
}

// registerRoutes registers the top-level endpoints with
// the router and calls the sub-handlers' router registration
// methods; must be called before Listen
func (s *Server) registerRoutes() {
	// Register base endpoints
	s.app.Get("/", s.index)
	s.app.Get("/contact", s.contact)
	s.app.All("/login/:provider", s.login)
	s.app.Get("/logout", s.logout)

	// Register the step handler and register its endpoints
	s.stepHandler = steps.New(s.app, s.toKeysProviders())
	s.stepHandler.Register()
}

// New creates a new instance of the HTTP server
func New(providers map[string]srvcommon.Provider) *Server {
	srv := &Server{
		app:       fiber.New(),
		store:     session.New(),
		providers: providers,
	}

	// Add middlewares
	srv.registerMiddlewares()

	// Register endpoints
	srv.registerRoutes()

	return srv
}

// Listen begins HTTP listening on the given address
func (s *Server) Listen(addr string) error {
	return s.app.Listen(addr)
}

// toKeysProviders converts the Providers stored in the server object to
// a map of their component SSHKeyProviders
func (s *Server) toKeysProviders() map[string]srvcommon.SSHKeyProvider {
	result := make(map[string]srvcommon.SSHKeyProvider, len(s.providers))
	for key, value := range s.providers {
		result[key] = value
	}
	return result
}

func (Server) index(ctx *fiber.Ctx) error {
	srvcommon.RespondWithHTML(ctx, templates.Index)
	return nil
}

func (Server) contact(ctx *fiber.Ctx) error {
	srvcommon.RespondWithHTML(ctx, templates.Contact)
	return nil
}
