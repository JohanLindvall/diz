package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/client"
)

func TestRestoreReadsDockerResponse(t *testing.T) {
	for _, tc := range []struct {
		name, response, wantError string
	}{
		{"success", "{\"stream\":\"Loaded image\\n\"}\n", ""},
		{"daemon failure", "{\"stream\":\"Loading\\n\"}\n{\"error\":\"missing image config\"}\n", "missing image config"},
		{"truncated response", "{\"stream\":", "unexpected EOF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/images/load") {
					t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.Path)
				}
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
				}
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, tc.response)
			}))
			defer server.Close()
			cli, err := client.NewClientWithOpts(client.WithHost(server.URL), client.WithVersion("1.48"))
			if err != nil {
				t.Fatal(err)
			}
			defer cli.Close()
			a := New(cli, Options{FromZip: writeDizFixture(t)})
			err = a.Restore([]string{"*"})
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Restore error = %v, want %q", err, tc.wantError)
			}
		})
	}
}
