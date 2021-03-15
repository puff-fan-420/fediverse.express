// Package srvcommon contains various constants, interfaces and
// functions used by one or more server provider
package srvcommon

import "math/rand"

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
