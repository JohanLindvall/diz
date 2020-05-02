package imagesource

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

func init() {
	for _, p := range []string{"$HOME/.docker/config.json", "$USERPROFILE/.docker/config.json"} {
		if b, e := ioutil.ReadFile(os.ExpandEnv(p)); e == nil {
			data := make(map[string]interface{}, 0)
			if err := json.Unmarshal(b, &data); err == nil {
				if auths, ok := data["auths"]; ok {
					if authsmap, ok := auths.(map[string]interface{}); ok {
						for k, iv := range authsmap {
							if v, ok := iv.(map[string]interface{}); ok {
								if auth, ok := v["auth"]; ok {
									if s, ok := auth.(string); ok {
										cache[k] = s
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

var cache = make(map[string]string, 0)

func getConfigCredentials(repository string) string {
	if a, ok := cache[repository]; ok {
		if dec, err := base64.StdEncoding.DecodeString(a); err == nil {
			if split := strings.Split(string(dec), ":"); len(split) == 2 {
				return marshalCredentials(split[0], split[1])
			}
		}
		return a
	}
	return ""
}
