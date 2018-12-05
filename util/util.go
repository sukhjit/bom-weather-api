package util

import (
	"math/rand"
	"regexp"
	"time"
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
)

var (
	re = regexp.MustCompile("[^a-z0-9]+")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandomString returns a random string with given length
func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
