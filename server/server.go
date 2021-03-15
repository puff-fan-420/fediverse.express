// Package server is responsible for the HTTP server that serves fediverse.express
package server

import (
	"fmt"
	"log"
	"strings"

	"github.com/CuteAP/fediverse.express/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

type Server struct {
	store     *session.Store
	providers map[string]Provider
	status    map[string]*Status // ???????????????
	app       *fiber.App
}

// New creates a new instance of the HTTP
// server
func New() (*Server, error) {
	srv := &Server{
		app:   fiber.New(),
		store: session.New(),
	}

	// Add middlewares
	// TODO: middlewares in respective registrations
	srv.app.Use(func(ctx *fiber.Ctx) error {
		session, err := srv.store.Get(ctx)
		if err != nil {
			log.Fatalf("Could not create session: %v", err)
		}

		ctx.Locals("session", session)
		ctx.Next()
		return nil
	})
	srv.app.Use(func(ctx *fiber.Ctx) error {
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

	// Register endpoints
	srv.app.Get("/", srv.index)
	srv.app.Get("/contact", srv.contact)
	srv.app.All("/login/:provider", srv.login)
}

func (Server) index(ctx *fiber.Ctx) error {
	respondWithHTML(ctx, templates.Index)
	return nil
}

func (Server) contact(ctx *fiber.Ctx) error {
	respondWithHTML(ctx, templates.Contact)
	return nil
}

type Status struct {
	Error error
	Done  bool
}

func respondWithHTML(ctx *fiber.Ctx, html string) {
	ctx.Status(200)
	ctx.Set("Content-Type", "text/html")
	ctx.SendString(fmt.Sprintf("%s %s %s", templates.Header, html, templates.Footer))
}
