package proxy_test

import (
	"testing"

	"github.com/mmdv/telman/cmd/internal/config/proxy"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		user    string
		pass    string
		wantErr bool
	}{
		{
			name:    "url: invalid scheme",
			url:     "80://host:80",
			user:    "",
			pass:    "",
			wantErr: true,
		},
		{
			name:    "url: supported scheme",
			url:     "http://host:80",
			user:    "",
			pass:    "",
			wantErr: false,
		},
		{
			name:    "url: supported scheme",
			url:     "https://host:80",
			user:    "",
			pass:    "",
			wantErr: false,
		},
		{
			name:    "url: supported scheme",
			url:     "socks5://host:80",
			user:    "",
			pass:    "",
			wantErr: false,
		},
		{
			name:    "url: unsupported scheme",
			url:     "ftp://host:80",
			user:    "",
			pass:    "",
			wantErr: true,
		},
		{
			name:    "url: missing scheme",
			url:     "host:80",
			user:    "",
			pass:    "",
			wantErr: true,
		},
		{
			name:    "url: empty string",
			url:     "",
			user:    "",
			pass:    "",
			wantErr: true,
		},
		{
			name:    "credentials: password without username",
			url:     "http://host:80",
			user:    "",
			pass:    "foo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := proxy.Validate(tt.url, tt.user, tt.pass)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
