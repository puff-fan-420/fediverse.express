package main

import (
	"math/rand"
	"time"

	"github.com/CuteAP/fediverse.express/server"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/oauth2"
)

const vals = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func SeedRNG() {
	rand.Seed(time.Now().UnixNano())
}

func RandomString(xlen int) string {
	vlen := len(vals)
	str := ""
	for len(str) < xlen {
		str = str + string(vals[rand.Intn(vlen)])
	}
	return str
}

func redirectWithState(prx server.Provider, ctx *fiber.Ctx) error {
	state := RandomString(15)

	session := ctx.Locals("session").(*session.Session)
	session.Set("state", state)
	session.Save()

	ctx.Redirect(prx.OAuth2().AuthCodeURL(state, oauth2.AccessTypeOnline))
	return nil
}

type Keys struct {
	PublicKey  []byte
	PrivateKey []byte
}
