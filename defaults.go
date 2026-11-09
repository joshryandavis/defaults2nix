package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// source is where `defaults` data comes from.
type source interface {
	Domains() ([]string, error)
	All() (string, error)
	Read(domain string) (string, error)
}

// defaultsSource runs the macOS `defaults` command.
type defaultsSource struct{}

func (defaultsSource) Domains() ([]string, error) {
	out, err := readDefaults("domains")
	if err != nil {
		return nil, err
	}
	return parseDomains(out), nil
}

// parseDomains reads the comma-separated list printed by `defaults domains`.
func parseDomains(out string) []string {
	var domains []string
	for _, domain := range strings.Split(out, ",") {
		if domain = strings.TrimSpace(domain); domain != "" {
			domains = append(domains, domain)
		}
	}
	return domains
}

func (defaultsSource) All() (string, error) { return readDefaults("read") }

func (defaultsSource) Read(domain string) (string, error) { return readDefaults("read", domain) }

func readDefaults(args ...string) (string, error) {
	out, err := exec.Command("defaults", args...).Output()
	if err == nil {
		return string(out), nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if message := strings.TrimSpace(string(exit.Stderr)); message != "" {
			return "", fmt.Errorf("defaults %s: %s", strings.Join(args, " "), message)
		}
	}
	return "", fmt.Errorf("defaults %s: %w", strings.Join(args, " "), err)
}
