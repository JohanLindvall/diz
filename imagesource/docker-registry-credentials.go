package imagesource

import (
	"encoding/base64"
	"encoding/json"
)

func marshalCredentials(username, password string) string {
	jsonBytes, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})

	return base64.StdEncoding.EncodeToString(jsonBytes)
}
