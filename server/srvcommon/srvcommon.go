// Package srvcommon contains various constants, interfaces and
// functions used by one or more server provider
package srvcommon

import (
	"fmt"
	"math/rand"

	"github.com/CuteAP/fediverse.express/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/oauth2"
)

// randAlphabet contains every character to be included in randomly generated strings
const randAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomString generates a string of xlen length from random alphabet runes
func RandomString(xlen int) string {
	vlen := len(randAlphabet)
	str := ""
	for len(str) < xlen {
		str = str + string(randAlphabet[rand.Intn(vlen)])
	}
	return str
}

// RespondWithHTML takes a given fiber request context and
// returns a 200 OK status with the given html response body
func RespondWithHTML(ctx *fiber.Ctx, html string) {
	ctx.Status(200)
	ctx.Set("Content-Type", "text/html")
	ctx.SendString(fmt.Sprintf("%s %s %s", templates.Header, html, templates.Footer))
}

// RedirectWithState sets a state cookie for OAuth and redirects the client
func RedirectWithState(oauther OAuther, ctx *fiber.Ctx) {
	state := RandomString(15)

	session := ctx.Locals("session").(*session.Session)
	session.Set("state", state)
	session.Save()

	ctx.Redirect(oauther.OAuth2().AuthCodeURL(state, oauth2.AccessTypeOnline))
}
