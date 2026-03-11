package rule

import (
	"fmt"
	"net"
	"strings"
)

func Matches(entry Entry, host string, port int) bool {
	if entry.Port != 0 && entry.Port != port {
		return false
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	entryHost := strings.ToLower(strings.TrimSuffix(entry.Host, "."))

	if strings.Contains(entryHost, "/") {
		_, ipNet, err := net.ParseCIDR(entryHost)
		if err != nil {
			return false
		}
		ip := net.ParseIP(host)
		return ip != nil && ipNet.Contains(ip)
	}

	if net.ParseIP(entryHost) != nil {
		return host == entryHost
	}

	if strings.HasPrefix(entryHost, "*.") {
		apex := entryHost[2:]
		if host == apex {
			return true
		}
		suffix := "." + apex
		return strings.HasSuffix(host, suffix) &&
			strings.Count(host, ".") == strings.Count(apex, ".")+1
	}

	return host == entryHost
}

func ValidateEntry(entry Entry) error {
	host := strings.TrimSpace(entry.Host)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	if entry.Port < 0 || entry.Port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535, got %d", entry.Port)
	}
	if strings.Contains(host, "://") {
		return fmt.Errorf("invalid host: scheme not allowed: %s", host)
	}
	if strings.ContainsAny(host, " \t") {
		return fmt.Errorf("invalid host: contains whitespace: %s", host)
	}

	if strings.Contains(host, "*") {
		wildcardCount := strings.Count(host, "*")
		if wildcardCount > 1 || !strings.HasPrefix(host, "*.") {
			return fmt.Errorf("invalid wildcard pattern: %s (only *.example.com form is allowed)", host)
		}
		if len(host) <= 2 {
			return fmt.Errorf("invalid wildcard pattern: %s (apex domain is empty)", host)
		}
		apex := host[2:]
		if err := validateHostname(apex); err != nil {
			return fmt.Errorf("invalid wildcard apex %q: %w", apex, err)
		}
		return nil
	}

	if strings.Contains(host, "/") {
		if _, _, err := net.ParseCIDR(host); err != nil {
			return fmt.Errorf("invalid CIDR: %s", host)
		}
		return nil
	}

	if net.ParseIP(host) != nil {
		return nil
	}

	return validateHostname(host)
}

func validateHostname(host string) error {
	if strings.Contains(host, "..") {
		return fmt.Errorf("invalid host: consecutive dots: %s", host)
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return fmt.Errorf("invalid host: leading or trailing dot: %s", host)
	}
	for _, label := range strings.Split(host, ".") {
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("invalid host: label starts or ends with '-': %s", host)
		}
	}
	return nil
}
