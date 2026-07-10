//go:build !windows
// +build !windows

package imagesource

func getCredentials(repository string) string {
	return getConfigCredentials(repository)
}
