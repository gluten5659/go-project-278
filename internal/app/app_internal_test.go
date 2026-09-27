package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const shortBaseURL = "https://short.example"

func TestParseBaseURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		rawBaseURL  string
		wantBaseURL string
		wantError   bool
	}{
		{
			name:        "keeps an https address",
			rawBaseURL:  shortBaseURL,
			wantBaseURL: shortBaseURL,
		},
		{
			name:        "keeps the port of a host",
			rawBaseURL:  "http://localhost:8080",
			wantBaseURL: "http://localhost:8080",
		},
		{
			name:        "trims surrounding spaces",
			rawBaseURL:  "  " + shortBaseURL + "  ",
			wantBaseURL: shortBaseURL,
		},
		{
			name:        "drops a trailing slash",
			rawBaseURL:  shortBaseURL + "/",
			wantBaseURL: shortBaseURL,
		},
		{
			name:       "rejects an empty value",
			rawBaseURL: "",
			wantError:  true,
		},
		{
			name:       "rejects an address without a scheme",
			rawBaseURL: "short.example",
			wantError:  true,
		},
		{
			name:       "rejects a scheme the browser cannot follow",
			rawBaseURL: "ftp://short.example",
			wantError:  true,
		},
		{
			name:       "rejects an address without a host",
			rawBaseURL: "https://",
			wantError:  true,
		},
		{
			name:       "rejects an address with a path",
			rawBaseURL: shortBaseURL + "/links",
			wantError:  true,
		},
		{
			name:       "rejects an address with a query",
			rawBaseURL: shortBaseURL + "?source=email",
			wantError:  true,
		},
		{
			name:       "rejects an address with a fragment",
			rawBaseURL: shortBaseURL + "#top",
			wantError:  true,
		},
		{
			name:       "rejects an address carrying a user",
			rawBaseURL: "https://someone@short.example",
			wantError:  true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			baseURL, err := parseBaseURL(testCase.rawBaseURL)

			if testCase.wantError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantBaseURL, baseURL)
		})
	}
}

func TestParseServerAddress(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		rawAddress  string
		wantAddress string
	}{
		{
			name:        "turns a bare port into an address",
			rawAddress:  "8080",
			wantAddress: ":8080",
		},
		{
			name:        "keeps an address that already has a colon",
			rawAddress:  ":9090",
			wantAddress: ":9090",
		},
		{
			name:        "keeps an address bound to one interface",
			rawAddress:  "127.0.0.1:8080",
			wantAddress: "127.0.0.1:8080",
		},
		{
			name:        "trims surrounding spaces",
			rawAddress:  "  8080  ",
			wantAddress: ":8080",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.wantAddress, parseServerAddress(testCase.rawAddress))
		})
	}
}
