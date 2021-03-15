package digitalocean

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/CuteAP/fediverse.express/server"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"golang.org/x/oauth2"
)

func hitEndpoint(method string, endpoint string, token string, body io.Reader, expectStatusCode int, response interface{}) error {
	req, err := http.NewRequest(method, fmt.Sprintf("https://api.digitalocean.com/v2/%s", endpoint), body)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("User-Agent", "catgirl")

	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := server.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	xb, err := io.ReadAll(resp.Body)
	if err != nil {
		xb = []byte("could not read request body")
	}

	if resp.StatusCode != expectStatusCode {
		return fmt.Errorf("expected status code %d, got %d %s", expectStatusCode, resp.StatusCode, xb)
	}

	return json.Unmarshal(xb, response)
}

type DigitalOcean struct{}

func (d *DigitalOcean) OAuth2() *oauth2.Config {
	return &oauth2.Config{
		RedirectURL:  fmt.Sprintf("%slogin/digitalocean", os.Getenv("CATGIRL_WEBROOT")),
		ClientID:     os.Getenv("CATGIRL_DIGITALOCEAN_CLIENT_ID"),
		ClientSecret: os.Getenv("CATGIRL_DIGITALOCEAN_CLIENT_SECRET"),
		Scopes:       []string{"read", "write"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://cloud.digitalocean.com/v1/oauth/authorize",
			TokenURL: "https://cloud.digitalocean.com/v1/oauth/token",
		},
	}
}

type DropletCreate struct {
	Name    string   `json:"name"`
	Region  string   `json:"region"`
	Size    string   `json:"size"`
	Image   string   `json:"image"`
	SSHKeys []int    `json:"ssh_keys"`
	Backups bool     `json:"backups"`
	IPv6    bool     `json:"ipv6"`
	Tags    []string `json:"tags"`
}

type Droplet struct {
	Droplet struct {
		ID       int64                       `json:"id"`
		Name     string                      `json:"name"`
		Locked   bool                        `json:"locked"`
		Status   string                      `json:"status"`
		Networks map[string][]DropletNetwork `json:"networks"`
	} `json:"droplet"`
}

type DropletNetwork struct {
	IP   string `json:"ip_address"`
	Type string `json:"type"`
}

type SSHKeyCreate struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type SSHKey struct {
	ID int `json:"id"`
}

type SSHKeyCreated struct {
	SSHKey struct {
		ID int `json:"id"`
	} `json:"ssh_key"`
}

var regions = []string{"nyc1", "nyc3", "sfo3"}

func (d *DigitalOcean) CreateServer(token string, sshKey interface{}) (*string, *string, error) {
	droplet := &DropletCreate{
		Name:    server.RandomString(10) + ".fediverse.express",
		Region:  regions[rand.Intn(len(regions))],
		Size:    "s-1vcpu-2gb",
		Image:   "ubuntu-20-04-x64",
		Backups: false,
		IPv6:    true,
		Tags:    []string{"fediverse.express"},
		SSHKeys: []int{sshKey.(int)},
	}

	jx, err := json.Marshal(droplet)
	if err != nil {
		return nil, nil, err
	}

	xdroplet := &Droplet{}
	err = hitEndpoint("POST", "droplets", token, bytes.NewReader(jx), 202, xdroplet)
	if err != nil {
		return nil, nil, fmt.Errorf("Droplet creation failed: %v", err)
	}

	for xdroplet.Droplet.Status != "active" {
		time.Sleep(2 * time.Second)

		err := hitEndpoint("GET", fmt.Sprintf("droplets/%d", xdroplet.Droplet.ID), token, nil, 200, xdroplet)
		if err != nil {
			return nil, nil, err
		}
	}

	ipv4, ipv6 := "", ""

	for _, ip := range xdroplet.Droplet.Networks["v4"] {
		if ip.Type == "public" {
			ipv4 = ip.IP
			break
		}
	}

	for _, ip := range xdroplet.Droplet.Networks["v6"] {
		if ip.Type == "public" {
			ipv6 = ip.IP
			break
		}
	}

	return &ipv4, &ipv6, err
}

func (d *DigitalOcean) CreateSSHKey(token string, sshKey string) (interface{}, error) {
	jx, err := json.Marshal(SSHKeyCreate{
		Name:      server.RandomString(10) + ".fediverse.express",
		PublicKey: sshKey,
	})
	if err != nil {
		return nil, err
	}

	key := SSHKeyCreated{}
	err = hitEndpoint("POST", "account/keys", token, bytes.NewReader(jx), 201, &key)
	if err != nil {
		return nil, err
	}

	return key.SSHKey.ID, nil
}

func (d *DigitalOcean) EnterCredentials() (string, map[string]string) {
	return "", make(map[string]string)
}

func (d *DigitalOcean) ValidateCredentials(ctx *fiber.Ctx, session *session.Session) error {
	return errors.New("not implemented")
}
