// Package domain is responsible for handling domains, such as domain validation
package domain

import (
	"errors"
	"log"
	"net"
	"strings"

	"github.com/asaskevich/govalidator"
)

// ValidateDomain validates a given domain, checks if it
// matches the given ipv4/ipv6 records, and returns an error
// if the domain is not valid for use in fediverse.express
func ValidateDomain(domain string, ipv4, ipv6 *string) error {
	if ipv4 == nil {
		return errors.New("no ipv4 on record")
	}

	ipv4Address := *ipv4
	var ipv6Address string

	if ipv6 == nil {
		// dereference as invalid
		ipv6Address = "invalid"
	} else {
		ipv6Address = *ipv6
	}

	if domain == "" {
		return errors.New("hostname is empty")
	}

	if !govalidator.IsDNSName(domain) {
		return errors.New("invalid domain name")
	}

	if strings.Index(domain, "_") > -1 {
		return errors.New("Let's Encrypt will not issue certificates for domains containing underscores")
	}

	addrs, err := net.LookupHost(domain)
	if err != nil {
		log.Printf("Error looking up %s: %v", domain, err)
		return errors.New("error looking up domain name")
	}

	if len(addrs) == 0 {
		return errors.New("host has no defined addresses. Did you set up your DNS correctly?")
	}

	for _, addr := range addrs {
		if addr != ipv4Address && addr != ipv6Address {
			log.Printf("Non-matching record %s", addr)
			return errors.New("found a non-matching record on your domain. Did you set up your DNS correctly?")
		}
	}

	return nil
}
