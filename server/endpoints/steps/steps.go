package steps

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/CuteAP/fediverse.express/server/domain"
	"github.com/CuteAP/fediverse.express/server/srvcommon"
	"github.com/CuteAP/fediverse.express/templates"
	ansibler "github.com/apenella/go-ansible"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/crypto/ssh"
)

type Steps struct {
	router   fiber.Router
	keys     map[string]srvcommon.SSHKeyProvider
	statuses map[string]*status
}

type status struct {
	Error error
	Done  bool
}

// New creates a new instance of the steps handler
func New(app *fiber.App, keyProviders map[string]srvcommon.SSHKeyProvider) *Steps {
	return &Steps{
		router:   app.Group("step"),
		keys:     keyProviders,
		statuses: make(map[string]*status),
	}
}

// Register registers the handler's endpoints to the
// app given to it in New
func (s *Steps) Register() {
	s.router.Add("GET", "download-key", s.downloadKey)
	s.router.Add("GET", "provision", s.getProvision)
	s.router.Add("POST", "provision", s.postProvision)
	s.router.Add("GET", "verify", s.getVerify)
	s.router.Add("POST", "verify", s.postVerify)
	s.router.Add("GET", "install", s.getInstall)
	s.router.Add("POST", "install", s.postInstall)
	s.router.Add("GET", "done", s.done)
}

func (Steps) downloadKey(ctx *fiber.Ctx) error {
	ctx.Set("Content-Type", "application/octet-stream")
	ctx.Set("Content-Disposition", "attachment; name=\"id_rsa\"; filename=\"id_rsa\"")

	session := ctx.Locals("session").(*session.Session)

	ctx.Write(pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(session.Get("privateKey").(*rsa.PrivateKey)),
	}))
	return nil
}

func (Steps) getProvision(ctx *fiber.Ctx) error {
	srvcommon.RespondWithHTML(ctx, templates.Provision)
	return nil
}

func (s *Steps) postProvision(ctx *fiber.Ctx) error {
	session := ctx.Locals("session").(*session.Session)

	publicKey, err := ssh.NewPublicKey(&session.Get("privateKey").(*rsa.PrivateKey).PublicKey)
	if err != nil {
		srvcommon.RespondWithHTML(ctx, fmt.Sprintf("Something went wrong when computing your private key. <a href='/login/%s'>Log in again</a> to generate a new one.", session.Get("provider")))
		return nil
	}

	token := session.Get("accessToken").(string)

	keyId, err := s.keys[session.Get("provider").(string)].CreateSSHKey(token, string(ssh.MarshalAuthorizedKey(publicKey)))
	if err != nil {
		log.Printf("Error adding SSH key: %v", err)
		srvcommon.RespondWithHTML(ctx, "Something went wrong adding the newly-created SSH key to your account. Check your provider's console and delete any SSH keys ending in '.fediverse.express' (or similar), then <form action='' method='post' style='display: inline;'><input type='submit' value='click here' /></form> to try again.")
		return nil
	}

	ipv4, ipv6, err := s.keys[session.Get("provider").(string)].CreateServer(token, keyId)
	if err != nil {
		log.Printf("Error provisioning server: %v", err)
		srvcommon.RespondWithHTML(ctx, "Something went wrong when provisioning your server. Check your provider's console to make sure a machine hasn't been created. If it has, delete/unprovision it and click <form action='' method='post' style='display: inline;'><input type='submit' value='here' /></form> to try again.")
		return nil
	}

	session.Set("ipv4", ipv4)
	session.Set("ipv6", ipv6)
	session.Save()

	ctx.Redirect("/step/verify")
	return nil
}

func (Steps) getVerify(ctx *fiber.Ctx) error {
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

	srvcommon.RespondWithHTML(ctx, fmt.Sprintf(templates.Verify, *session.Get("ipv4").(*string), *ipv6))
	return nil
}

func (Steps) postVerify(ctx *fiber.Ctx) error {
	session := ctx.Locals("session").(*session.Session)

	if session.Get("ipv4") == nil {
		ctx.Redirect("/step/provision")
		return nil
	}

	input := &struct {
		Hostname string
	}{}

	err := ctx.BodyParser(input)
	if err != nil {
		return errors.New("invalid form body")
	}

	ipv4 := session.Get("ipv4").(*string)
	ipv6 := session.Get("ipv6").(*string)

	if err := domain.ValidateDomain(input.Hostname, ipv4, ipv6); err != nil {
		ipv6, ok := session.Get("ipv6").(*string)
		if !ok || ipv6 == nil {
			na := "not applicable"
			ipv6 = &na
		}

		srvcommon.RespondWithHTML(ctx, "<b>Error:</b> "+err.Error()+"<br><br>"+fmt.Sprintf(templates.Verify, *session.Get("ipv4").(*string), *ipv6))
		return nil
	}

	session.Set("hostname", input.Hostname)
	session.Save()

	ctx.Redirect("/step/install")
	return nil
}

func (s *Steps) getInstall(ctx *fiber.Ctx) error {
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

	if sx, ok := s.statuses[ipv4]; ok {
		if sx.Done {
			ctx.Redirect("/step/done")
			return nil
		}

		if sx.Error != nil {
			delete(s.statuses, ipv4)

			srvcommon.RespondWithHTML(ctx, "<b>Error:</b> "+sx.Error.Error()+"<br><br>"+templates.Install)
			return nil
		}
		srvcommon.RespondWithHTML(ctx, templates.Running)
		return nil
	}
	srvcommon.RespondWithHTML(ctx, templates.Install)
	return nil
}

func (s *Steps) postInstall(ctx *fiber.Ctx) error {
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
	ipv6 := session.Get("ipv6").(*string)

	if err := domain.ValidateDomain(session.Get("hostname").(string), ipv4, ipv6); err != nil {
		ctx.Redirect("/step/verify")
		return nil
	}

	if sx, ok := s.statuses[*ipv4]; ok {
		if !sx.Done && sx.Error != nil {
			ctx.Redirect("/step/install")
			return nil
		}
	}

	ex := func(html string) error {
		srvcommon.RespondWithHTML(ctx, html+"<br><br>"+templates.Install)
		return nil
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
		s.statuses[*ipv4] = &status{
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

			s.statuses[*ipv4].Error = fmt.Errorf("There was an error preparing your instance. Check that your server is on and working and try again. If this error persists, please e-mail us so we can help you out.")
			return
		}

		s.statuses[*ipv4].Done = true
	}()

	time.Sleep(2 * time.Second)
	ctx.Redirect("/step/install")
	return nil
}

func (s *Steps) done(ctx *fiber.Ctx) error {
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

	if sx, ok := s.statuses[*ipv4]; ok {
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

	srvcommon.RespondWithHTML(ctx, fmt.Sprintf(templates.Done, session.Get("hostname").(string), *ipv4, string(pk), string(pk)))
	return nil
}
