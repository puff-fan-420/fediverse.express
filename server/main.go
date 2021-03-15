package server

import (
	"math/rand"
	"net/http"
)

var HTTPClient = http.Client{}

const vals = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandomString(xlen int) string {
	vlen := len(vals)
	str := ""
	for len(str) < xlen {
		str = str + string(vals[rand.Intn(vlen)])
	}
	return str
}
