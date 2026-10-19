package analyzer

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid https", "https://example.com", nil},
		{"valid http with port and path", "http://example.com:8080/a/b?x=1", nil},
		{"surrounding whitespace is trimmed", "  https://example.com  ", nil},
		{"empty", "", ErrEmptyInput},
		{"whitespace only", " \t\n", ErrEmptyInput},
		{"no scheme", "example.com/path", ErrInvalidURL},
		{"unsupported scheme", "ftp://example.com", ErrInvalidURL},
		{"missing host", "https:///path", ErrInvalidURL},
		{"port zero", "https://example.com:0", ErrInvalidURL},
		{"port too large", "https://example.com:70000", ErrInvalidURL},
		{"non numeric port", "https://example.com:abc", ErrInvalidURL},
		{"unparsable", "http://exa mple.com", ErrInvalidURL},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := URLAnalyzer{}.Validate(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		scheme, host string
		port         int
		defaultPort  bool
		isIP         bool
		segments     []string
		query        map[string][]string
		fragment     string
		secure       bool
		credentials  bool
		warnings     []string
	}{
		{
			name:   "plain https",
			input:  "https://Example.COM/docs/go/?lang=en&lang=ru#intro",
			scheme: "https", host: "example.com", port: 443, defaultPort: true,
			segments: []string{"docs", "go"},
			query:    map[string][]string{"lang": {"en", "ru"}},
			fragment: "intro",
			secure:   true,
		},
		{
			name:   "root path has no segments",
			input:  "https://example.com",
			scheme: "https", host: "example.com", port: 443, defaultPort: true,
			query:  map[string][]string{},
			secure: true,
		},
		{
			name:   "risky url",
			input:  "http://admin:pw@192.168.0.10:8080/login?token=abc",
			scheme: "http", host: "192.168.0.10", port: 8080, isIP: true,
			segments:    []string{"login"},
			query:       map[string][]string{"token": {"abc"}},
			credentials: true,
			warnings: []string{
				"uses unencrypted http",
				"contains credentials in the URL",
				"non-default port 8080",
				"host is a raw IP address",
				`sensitive query parameter "token"`,
			},
		},
		{
			name:   "explicit default port",
			input:  "http://example.com:80/",
			scheme: "http", host: "example.com", port: 80, defaultPort: true,
			query:    map[string][]string{},
			warnings: []string{"uses unencrypted http"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := URLAnalyzer{}.Analyze(tc.input)
			if err != nil {
				t.Fatalf("Analyze(%q) unexpected error: %v", tc.input, err)
			}
			check := func(field string, got, want any) {
				t.Helper()
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %#v, want %#v", field, got, want)
				}
			}
			check("Scheme", got.Scheme, tc.scheme)
			check("Host", got.Host, tc.host)
			check("Port", got.Port, tc.port)
			check("DefaultPort", got.DefaultPort, tc.defaultPort)
			check("IsIP", got.IsIP, tc.isIP)
			check("PathSegments", got.PathSegments, tc.segments)
			check("Query", got.Query, tc.query)
			check("Fragment", got.Fragment, tc.fragment)
			check("Secure", got.Secure, tc.secure)
			check("HasCredentials", got.HasCredentials, tc.credentials)
			check("Warnings", got.Warnings, tc.warnings)
		})
	}
}

func TestAnalyzeRejectsInvalidInput(t *testing.T) {
	if _, err := (URLAnalyzer{}).Analyze(""); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("error = %v, want ErrEmptyInput", err)
	}
}
