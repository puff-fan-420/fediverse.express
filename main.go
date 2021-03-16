package main

import (
	_ "embed"
	"math/rand"
	"time"

	"github.com/CuteAP/fediverse.express/server"
	"github.com/CuteAP/fediverse.express/server/aws"
	"github.com/CuteAP/fediverse.express/server/digitalocean"
	"github.com/CuteAP/fediverse.express/server/srvcommon"
	"github.com/joho/godotenv"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	godotenv.Load()

	providers := map[string]srvcommon.Provider{
		"digitalocean": &digitalocean.DigitalOcean{},
		"aws":          &aws.AWS{},
	}
	// Initialize the HTTP server and begin listening
	srv := server.New(providers)

	srv.Listen(":4000")
}
