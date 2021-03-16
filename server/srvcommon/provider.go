package srvcommon

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/oauth2"
)

type Provider interface {
	OAuther
	SSHKeyProvider
	CredentialProvider
}

type OAuther interface {
	OAuth2() *oauth2.Config
}

type SSHKeyProvider interface {
	CreateSSHKey(token string, sshKey string) (interface{}, error)
	CreateServer(token string, sshKey interface{}) (*string, *string, error)
}

type CredentialProvider interface {
	EnterCredentials() (string, map[string]string)
	ValidateCredentials(ctx *fiber.Ctx, session *session.Session) error
}
