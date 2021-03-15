package main

import (
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/CuteAP/fediverse.express/server"
	"github.com/CuteAP/fediverse.express/server/aws"
	"github.com/CuteAP/fediverse.express/server/digitalocean"
	"github.com/CuteAP/fediverse.express/templates"
	ansibler "github.com/apenella/go-ansible"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

var (
	store *session.Store

	providers map[string]server.Provider = map[string]server.Provider{
		"digitalocean": &digitalocean.DigitalOcean{},
		"aws":          &aws.AWS{},
	}

	status map[string]*Status = make(map[string]*Status)
)

func main() {
	godotenv.Load()

	SeedRNG()

	app := fiber.New()
	store = session.New()

	app.Use(func(ctx *fiber.Ctx) error {
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

	app.Get("/logout", func(ctx *fiber.Ctx) error {
		ctx.Locals("session").(*session.Session).Destroy()
		ctx.Redirect("/")
		return nil
	})

	app.Get("/step/download-key", func(ctx *fiber.Ctx) error {
		ctx.Set("Content-Type", "application/octet-stream")
		ctx.Set("Content-Disposition", "attachment; name=\"id_rsa\"; filename=\"id_rsa\"")

		session := ctx.Locals("session").(*session.Session)

		ctx.Write(pem.EncodeToMemory(&pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   x509.MarshalPKCS1PrivateKey(session.Get("privateKey").(*rsa.PrivateKey)),
		}))
		return nil
	})

	app.Get("/step/provision", func(ctx *fiber.Ctx) error {
		return respondWithHTML(ctx, templates.Provision)
	})

	app.Post("/step/provision", func(ctx *fiber.Ctx) error {
		session := ctx.Locals("session").(*session.Session)

		publicKey, err := ssh.NewPublicKey(&session.Get("privateKey").(*rsa.PrivateKey).PublicKey)
		if err != nil {
			return respondWithHTML(ctx, fmt.Sprintf("Something went wrong when computing your private key. <a href='/login/%s'>Log in again</a> to generate a new one.", session.Get("provider")))
		}

		token := session.Get("accessToken").(string)

		keyId, err := providers[session.Get("provider").(string)].CreateSSHKey(token, string(ssh.MarshalAuthorizedKey(publicKey)))
		if err != nil {
			log.Printf("Error adding SSH key: %v", err)
			return respondWithHTML(ctx, "Something went wrong adding the newly-created SSH key to your account. Check your provider's console and delete any SSH keys ending in '.fediverse.express' (or similar), then <form action='' method='post' style='display: inline;'><input type='submit' value='click here' /></form> to try again.")
		}

		ipv4, ipv6, err := providers[session.Get("provider").(string)].CreateServer(token, keyId)
		if err != nil {
			log.Printf("Error provisioning server: %v", err)
			return respondWithHTML(ctx, "Something went wrong when provisioning your server. Check your provider's console to make sure a machine hasn't been created. If it has, delete/unprovision it and click <form action='' method='post' style='display: inline;'><input type='submit' value='here' /></form> to try again.")
		}

		session.Set("ipv4", ipv4)
		session.Set("ipv6", ipv6)
		session.Save()

		ctx.Redirect("/step/verify")
		return nil
	})

	app.Get("/step/verify", func(ctx *fiber.Ctx) error {
		session := ctx.Locals("session").(*session.Session)

		if session.Get("ipv4") == nil {
			ctx.Redirect("/step/provision")
			return nil
		}

		ipv6, ok := session.Get("ipv6").(*string)
		if !ok || ipv6 == nil {
			na := "not applicable"
			ipv6 = &na
		}

		return respondWithHTML(ctx, fmt.Sprintf(templates.Verify, *session.Get("ipv4").(*string), *ipv6))
	})

	app.Post("/step/verify", func(ctx *fiber.Ctx) error {
		session := ctx.Locals("session").(*session.Session)

		if session.Get("ipv4") == nil {
			ctx.Redirect("/step/provision")
			return nil
		}

		input := &InstallStartInput{}
		err := ctx.BodyParser(input)
		if err != nil {
			return errors.New("invalid form body")
		}

		if err := verifyDomain(ctx, input.Hostname); err != nil {
			ipv6, ok := session.Get("ipv6").(*string)
			if !ok || ipv6 == nil {
				na := "not applicable"
				ipv6 = &na
			}

			return respondWithHTML(ctx, "<b>Error:</b> "+err.Error()+"<br><br>"+fmt.Sprintf(templates.Verify, *session.Get("ipv4").(*string), *ipv6))
		}

		session.Set("hostname", input.Hostname)
		session.Save()

		ctx.Redirect("/step/install")
		return nil
	})

	app.Get("/step/install", func(ctx *fiber.Ctx) error {
		session := ctx.Locals("session").(*session.Session)

		if session.Get("ipv4") == nil {
			ctx.Redirect("/step/provision")
			return nil
		}

		if session.Get("hostname") == nil {
			ctx.Redirect("/step/verify")
			return nil
		}

		ipv4 := *session.Get("ipv4").(*string)

		if sx, ok := status[ipv4]; ok {
			if sx.Done {
				ctx.Redirect("/step/done")
				return nil
			}

			if sx.Error != nil {
				delete(status, ipv4)

				return respondWithHTML(ctx, "<b>Error:</b> "+sx.Error.Error()+"<br><br>"+templates.Install)
			}

			return respondWithHTML(ctx, templates.Running)
		}

		return respondWithHTML(ctx, templates.Install)
	})

	app.Post("/step/install", func(ctx *fiber.Ctx) error {
		session := ctx.Locals("session").(*session.Session)

		if session.Get("ipv4") == nil {
			ctx.Redirect("/step/provision")
			return nil
		}

		if session.Get("hostname") == nil {
			ctx.Redirect("/step/verify")
			return nil
		}

		if err := verifyDomain(ctx, session.Get("hostname").(string)); err != nil {
			ctx.Redirect("/step/verify")
			return nil
		}

		ipv4 := session.Get("ipv4").(*string)

		if sx, ok := status[*ipv4]; ok {
			if !sx.Done && sx.Error != nil {
				ctx.Redirect("/step/install")
				return nil
			}
		}

		ex := func(html string) error {
			return respondWithHTML(ctx, html+"<br><br>"+templates.Install)
		}

		// write ssh key
		fx, err := os.Create(*ipv4 + ".key")
		if err != nil {
			log.Printf("Error creating file %s: %v", *ipv4+".key", err)
			return ex("An internal server error occured. Please try again.")
		}
		defer fx.Close()

		os.Chmod(*ipv4+".key", 0600)

		err = pem.Encode(fx, &pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   x509.MarshalPKCS1PrivateKey(session.Get("privateKey").(*rsa.PrivateKey)),
		})
		if err != nil {
			log.Printf("Error writing to file %s: %v", *ipv4+".key", err)
			return ex("An internal server error occured. Please try again.")
		}

		provider := "" + session.Get("provider").(string)

		go func() {
			status[*ipv4] = &Status{
				Error: nil,
				Done:  false,
			}

			defer func() {
				err := os.Remove(*ipv4 + ".key")
				if err != nil {
					log.Printf("Error removing private key: %v", err)
				}
				err = os.Remove("catgirl/.catgirl/" + session.Get("hostname").(string) + "/postgresql")
				if err != nil {
					log.Printf("Error removing postgresql password: %v", err)
				}
				err = os.Remove("catgirl/.catgirl/" + session.Get("hostname").(string))
				if err != nil {
					log.Printf("Error removing catgirl settings directory: %v", err)
				}
			}()

			// having nice things is STILL not allowed
			user := "root"
			if provider == "aws" {
				user = "ubuntu"
			}

			ansible := ansibler.AnsiblePlaybookCmd{
				CmdRunDir: "catgirl",
				Playbook:  "main.yml",
				Options: &ansibler.AnsiblePlaybookOptions{
					ExtraVars: map[string]interface{}{
						"domain": session.Get("hostname").(string),
						"email":  "tb@gamers.exposed",
					},
					Inventory: *ipv4 + ",",
				},
				ConnectionOptions: &ansibler.AnsiblePlaybookConnectionOptions{
					AskPass:    false,
					User:       user,
					PrivateKey: "../" + *ipv4 + ".key",
				},
				Writer: os.Stdout,
			}

			ansibler.AnsibleAvoidHostKeyChecking()

			err = ansible.Run()
			if err != nil {
				log.Printf("Ansible exited with error: %v", err)

				status[*ipv4].Error = fmt.Errorf("There was an error preparing your instance. Check that your server is on and working and try again. If this error persists, please e-mail us so we can help you out.")
				return
			}

			status[*ipv4].Done = true
		}()

		time.Sleep(2 * time.Second)
		ctx.Redirect("/step/install")
		return nil
	})

	app.Get("/step/done", func(ctx *fiber.Ctx) error {
		session := ctx.Locals("session").(*session.Session)

		if session.Get("ipv4") == nil {
			ctx.Redirect("/step/provision")
			return nil
		}

		if session.Get("hostname") == nil {
			ctx.Redirect("/step/verify")
			return nil
		}

		ipv4 := session.Get("ipv4").(*string)

		if sx, ok := status[*ipv4]; ok {
			if !sx.Done && sx.Error != nil {
				ctx.Redirect("/step/install")
				return nil
			}
		}

		pk := pem.EncodeToMemory(&pem.Block{
			Type:    "RSA PRIVATE KEY",
			Headers: nil,
			Bytes:   x509.MarshalPKCS1PrivateKey(session.Get("privateKey").(*rsa.PrivateKey)),
		})

		return respondWithHTML(ctx, fmt.Sprintf(templates.Done, session.Get("hostname").(string), *ipv4, string(pk), string(pk)))
	})

	app.Listen(":4000")
}
