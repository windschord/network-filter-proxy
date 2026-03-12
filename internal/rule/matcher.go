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
	host = NormalizeHost(host)
	entryHost := NormalizeHost(entry.Host)

	if strings.Contains(entryHost, "/") {
		_, ipNet, err := net.ParseCIDR(entryHost)
		if err != nil {
			return false
		}
		ip := net.ParseIP(host)
		return ip != nil && ipNet.Contains(ip)
	}

	if ipEntry := net.ParseIP(entryHost); ipEntry != nil {
		ipHost := net.ParseIP(host)
		if ipHost == nil {
			return false
		}
		return ipHost.Equal(ipEntry)
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

// ValidationError provides structured validation error info.
type ValidationError struct {
	Field   string // "host" or "port"
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NormalizeHost normalizes a host string by trimming whitespace, removing
// trailing dots (FQDN), and converting to lowercase. Used by API handler
// to store normalized values consistent with Matches() normalization.
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimSuffix(host, ".")
	return strings.ToLower(host)
}

func ValidateEntry(entry Entry) error {
	host := NormalizeHost(entry.Host)
	if host == "" {
		return &ValidationError{Field: "host", Message: "host is required"}
	}
	if entry.Port < 0 || entry.Port > 65535 {
		return &ValidationError{Field: "port", Message: fmt.Sprintf("port must be between 0 and 65535, got %d", entry.Port)}
	}
	if err := rejectBadHostChars(host); err != nil {
		return err
	}

	if strings.Contains(host, "*") {
		wildcardCount := strings.Count(host, "*")
		if wildcardCount > 1 || !strings.HasPrefix(host, "*.") {
			return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid wildcard pattern: %s (only *.example.com form is allowed)", host)}
		}
		if len(host) <= 2 {
			return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid wildcard pattern: %s (apex domain is empty)", host)}
		}
		apex := host[2:]
		if err := validateHostname(apex); err != nil {
			return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid wildcard apex %q: %s", apex, err)}
		}
		return nil
	}

	if strings.Contains(host, "/") {
		if _, _, err := net.ParseCIDR(host); err != nil {
			return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid CIDR: %s", host)}
		}
		return nil
	}

	if net.ParseIP(host) != nil {
		return nil
	}

	if err := validateHostname(host); err != nil {
		return &ValidationError{Field: "host", Message: err.Error()}
	}
	return nil
}

func rejectBadHostChars(host string) *ValidationError {
	if strings.Contains(host, "://") {
		return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid host: scheme not allowed: %s", host)}
	}
	if strings.ContainsAny(host, " \t\n\r") {
		return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid host: contains whitespace: %s", host)}
	}
	// Reject host:port format (port is a separate field).
	if _, _, err := net.SplitHostPort(host); err == nil {
		return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid host: must not contain port: %s", host)}
	}
	// Reject bare colons in non-IP hosts (e.g. "example.com:").
	if strings.Contains(host, ":") && net.ParseIP(host) == nil && !strings.Contains(host, "/") {
		return &ValidationError{Field: "host", Message: fmt.Sprintf("invalid host: unexpected colon: %s", host)}
	}
	return nil
}

func validateHostname(host string) error {
	if len(host) > 253 {
		return fmt.Errorf("invalid host: exceeds 253 characters: %s", host[:50]+"...")
	}
	if strings.Contains(host, "..") {
		return fmt.Errorf("invalid host: consecutive dots: %s", host)
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return fmt.Errorf("invalid host: leading or trailing dot: %s", host)
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) > 63 {
			return fmt.Errorf("invalid host: label exceeds 63 characters: %s", host)
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("invalid host: label starts or ends with '-': %s", host)
		}
		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' && c != '*' {
				return fmt.Errorf("invalid host: invalid character %q in label: %s", c, host)
			}
		}
	}
	return nil
}
