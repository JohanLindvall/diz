// +build windows

package imagesource

import (
	"github.com/docker/docker-credential-helpers/wincred"
)

var (
	cred = wincred.Wincred{}
)

func getCredentials(url string) (string, string, error) {
	return cred.Get(url)
}
