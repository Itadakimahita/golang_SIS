package analyzer

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"sis/internal/task"
)

var (
	ErrEmptyInput = errors.New("url is empty")

	ErrInvalidURL = errors.New("invalid url")
)

type Analyzer interface {
	Validate(input string) error
	Analyze(input string) (task.Result, error)
}

type URLAnalyzer struct{}

var _ Analyzer = URLAnalyzer{}

var defaultPorts = map[string]int{"http": 80, "https": 443}

var sensitiveParams = []string{"password", "passwd", "token", "secret", "api_key", "apikey"}

func parse(input string) (*url.URL, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrEmptyInput
	}
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if _, ok := defaultPorts[scheme]; !ok {
		return nil, fmt.Errorf("%w: scheme must be http or https, got %q", ErrInvalidURL, u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("%w: port %q out of range 1-65535", ErrInvalidURL, p)
		}
	}
	return u, nil
}

func (URLAnalyzer) Validate(input string) error {
	_, err := parse(input)
	return err
}

func (URLAnalyzer) Analyze(input string) (task.Result, error) {
	u, err := parse(input)
	if err != nil {
		return task.Result{}, err
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())

	res := task.Result{
		Scheme:         scheme,
		Host:           host,
		Port:           defaultPorts[scheme],
		DefaultPort:    true,
		Path:           u.EscapedPath(),
		Query:          map[string][]string(u.Query()),
		Fragment:       u.Fragment,
		Secure:         scheme == "https",
		HasCredentials: u.User != nil,
	}
	if p := u.Port(); p != "" {
		res.Port, _ = strconv.Atoi(p)
		res.DefaultPort = res.Port == defaultPorts[scheme]
	}
	_, ipErr := netip.ParseAddr(host)
	res.IsIP = ipErr == nil

	for _, seg := range strings.Split(res.Path, "/") {
		if seg != "" {
			res.PathSegments = append(res.PathSegments, seg)
		}
	}

	if !res.Secure {
		res.Warnings = append(res.Warnings, "uses unencrypted http")
	}
	if res.HasCredentials {
		res.Warnings = append(res.Warnings, "contains credentials in the URL")
	}
	if !res.DefaultPort {
		res.Warnings = append(res.Warnings, fmt.Sprintf("non-default port %d", res.Port))
	}
	if res.IsIP {
		res.Warnings = append(res.Warnings, "host is a raw IP address")
	}
	for _, key := range sensitiveParams {
		if _, ok := res.Query[key]; ok {
			res.Warnings = append(res.Warnings, fmt.Sprintf("sensitive query parameter %q", key))
		}
	}
	return res, nil
}
