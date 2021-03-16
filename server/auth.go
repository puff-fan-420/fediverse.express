package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/CuteAP/fediverse.express/server/srvcommon"
	"github.com/CuteAP/fediverse.express/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func (s *Server) login(ctx *fiber.Ctx) error {
	prov, ok := s.providers[ctx.Params("provider", "invalid")]

	if !ok {
		log.Printf("Client requested unimplemented provider %s", ctx.Params("provider"))
		ctx.Redirect("/")
		return nil
	}

	session := ctx.Locals("session").(*session.Session)

	if prov.OAuth2() != nil {
		st := session.Get("state")
		if st == nil || st != ctx.Query("state", "invalid") || ctx.Query("code") == "" {
			log.Printf("Error: could not find one of state, code. st=%s, state=%s, code nil? %t", st, ctx.Query("state", "invalid"), ctx.Query("code") == "")
			srvcommon.RedirectWithState(prov, ctx)
			return nil
		}

		token, err := prov.OAuth2().Exchange(context.Background(), ctx.Query("code"))
		if err != nil {
			log.Printf("Error exchanging code: %v", err)
			srvcommon.RedirectWithState(prov, ctx)
			return nil
		}

		session.Set("provider", ctx.Params("provider"))
		session.Set("accessToken", token.AccessToken)
	} else {
		var err error

		if ctx.Method() == "POST" {
			err = prov.ValidateCredentials(ctx, session)
		} else {
			// lol
			err = errors.New("Log in with the following instructions:")
		}

		if err != nil {
			info, items := prov.EnterCredentials()

			cx := ""
			for i, desc := range items {
				cx += fmt.Sprintf("<b>%s</b> <input type='password' name='%s' /><br>", desc, i)
			}

			srvcommon.RespondWithHTML(ctx, err.Error()+"<br><br>"+fmt.Sprintf(templates.Prov, info, cx))
			return nil
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil || privateKey.Validate() != nil {
		log.Fatalf("Could not generate RSA keys: %v", err)
		return nil
	}

	session.Set("privateKey", privateKey)

	session.Save()

	if session.Get("step") != nil {
		ctx.Redirect(strings.Replace("step/"+session.Get("step").(string), "/", "", -1))
		return nil
	}

	ctx.Redirect("/step/provision")
	return nil
}

func (Server) logout(ctx *fiber.Ctx) error {
	ctx.Locals("session").(*session.Session).Destroy()
	ctx.Redirect("/")
	return nil
}
