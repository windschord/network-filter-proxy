package rule_test

import (
	"testing"

	"github.com/claudework/network-filter-proxy/internal/rule"
)

func TestMatches_ExactDomain(t *testing.T) {
	entry := rule.Entry{Host: "api.anthropic.com", Port: 443}
	if !rule.Matches(entry, "api.anthropic.com", 443) {
		t.Error("expected match for exact domain")
	}
}

func TestMatches_ExactDomain_PortMismatch(t *testing.T) {
	entry := rule.Entry{Host: "api.anthropic.com", Port: 443}
	if rule.Matches(entry, "api.anthropic.com", 80) {
		t.Error("expected no match for port mismatch")
	}
}

func TestMatches_Wildcard_Apex(t *testing.T) {
	entry := rule.Entry{Host: "*.github.com", Port: 443}
	if !rule.Matches(entry, "github.com", 443) {
		t.Error("expected wildcard to match apex domain")
	}
}

func TestMatches_Wildcard_Subdomain(t *testing.T) {
	entry := rule.Entry{Host: "*.github.com", Port: 443}
	if !rule.Matches(entry, "api.github.com", 443) {
		t.Error("expected wildcard to match single subdomain")
	}
}

func TestMatches_Wildcard_MultiLevel_NoMatch(t *testing.T) {
	entry := rule.Entry{Host: "*.github.com", Port: 443}
	if rule.Matches(entry, "evil.api.github.com", 443) {
		t.Error("expected no match for multi-level subdomain")
	}
}

func TestMatches_IPExact(t *testing.T) {
	entry := rule.Entry{Host: "140.82.112.3", Port: 443}
	if !rule.Matches(entry, "140.82.112.3", 443) {
		t.Error("expected match for exact IP")
	}
}

func TestMatches_CIDR_Match(t *testing.T) {
	entry := rule.Entry{Host: "140.82.112.0/20", Port: 443}
	if !rule.Matches(entry, "140.82.112.3", 443) {
		t.Error("expected CIDR match")
	}
}

func TestMatches_CIDR_NoMatch(t *testing.T) {
	entry := rule.Entry{Host: "140.82.112.0/20", Port: 443}
	if rule.Matches(entry, "8.8.8.8", 443) {
		t.Error("expected no CIDR match for out-of-range IP")
	}
}

func TestMatches_PortZero_AllowsAny(t *testing.T) {
	entry := rule.Entry{Host: "example.com", Port: 0}
	if !rule.Matches(entry, "example.com", 8080) {
		t.Error("expected port 0 to allow any port")
	}
	if !rule.Matches(entry, "example.com", 443) {
		t.Error("expected port 0 to allow any port")
	}
}

func TestMatches_NormalizeUppercase(t *testing.T) {
	entry := rule.Entry{Host: "api.anthropic.com", Port: 443}
	if !rule.Matches(entry, "API.ANTHROPIC.COM", 443) {
		t.Error("expected case-insensitive match")
	}
}

func TestMatches_NormalizeTrailingDot(t *testing.T) {
	entry := rule.Entry{Host: "api.anthropic.com", Port: 443}
	if !rule.Matches(entry, "api.anthropic.com.", 443) {
		t.Error("expected match with trailing dot")
	}
}

func TestValidateEntry_EmptyHost(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "", Port: 443})
	if err == nil {
		t.Error("expected error for empty host")
	}
}

func TestValidateEntry_MultiLevelWildcard(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "*.*.example.com", Port: 443})
	if err == nil {
		t.Error("expected error for multi-level wildcard")
	}
}

func TestValidateEntry_EmptyWildcardApex(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "*.", Port: 443})
	if err == nil {
		t.Error("expected error for empty wildcard apex")
	}
}

func TestValidateEntry_SchemeInHost(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "https://github.com", Port: 443})
	if err == nil {
		t.Error("expected error for scheme in host")
	}
}

func TestValidateEntry_ConsecutiveDots(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "api..github.com", Port: 443})
	if err == nil {
		t.Error("expected error for consecutive dots")
	}
}

func TestValidateEntry_InvalidPort(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "example.com", Port: 99999})
	if err == nil {
		t.Error("expected error for port > 65535")
	}
}

func TestValidateEntry_NegativePort(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "example.com", Port: -1})
	if err == nil {
		t.Error("expected error for negative port")
	}
}

func TestValidateEntry_ValidCIDR(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "10.0.0.0/8", Port: 443})
	if err != nil {
		t.Errorf("unexpected error for valid CIDR: %v", err)
	}
}

func TestValidateEntry_InvalidCIDR(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "10.0.0.0/99", Port: 443})
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}
}

func TestValidateEntry_ValidDomain(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "api.github.com", Port: 443})
	if err != nil {
		t.Errorf("unexpected error for valid domain: %v", err)
	}
}

func TestValidateEntry_ValidWildcard(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "*.github.com", Port: 443})
	if err != nil {
		t.Errorf("unexpected error for valid wildcard: %v", err)
	}
}

func TestValidateEntry_ValidIP(t *testing.T) {
	err := rule.ValidateEntry(rule.Entry{Host: "140.82.112.3", Port: 443})
	if err != nil {
		t.Errorf("unexpected error for valid IP: %v", err)
	}
}
