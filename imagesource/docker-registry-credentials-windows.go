// +build windows

package imagesource

import (
	"github.com/docker/docker-credential-helpers/wincred"
)

var (
	cred = wincred.Wincred{}
)

func getCredentials(repository string) string {
	if u, p, e := cred.Get(repository); e == nil {
		return marshalCredentials(u, p)
	} else {
		return getConfigCredentials(repository)
	}
}
