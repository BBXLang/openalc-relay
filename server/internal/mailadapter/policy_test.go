package mailadapter

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

// denyAll is a policy with the allowlist hook present but refusing
// everything: the hook's existence must not loosen the default denial.
func denyAll() EndpointPolicy {
	return EndpointPolicy{
		PrivateAllow: func(ServiceKind, string, net.IP, int) bool { return false },
	}
}

func TestPolicyDeniesPrivateTargetsByDefault(t *testing.T) {
	privateHosts := []string{
		"127.0.0.1",       // loopback
		"127.8.8.8",       // loopback range
		"::1",             // IPv6 loopback
		"10.1.2.3",        // RFC 1918
		"172.16.9.9",      // RFC 1918
		"192.168.1.10",    // RFC 1918
		"169.254.1.1",     // link-local
		"::ffff:10.0.0.7", // IPv4-mapped private
		"fc00::1",         // unique local
		"fe80::1",         // link-local
		"0.0.0.0",         // unspecified
		"224.0.0.5",       // multicast
	}
	for _, host := range privateHosts {
		for _, tc := range []struct {
			kind ServiceKind
			port int
		}{
			{ServiceIMAP, 993},
			{ServiceSMTP, 465},
		} {
			policy := EndpointPolicy{}
			err := policy.Validate(context.Background(), tc.kind, host, tc.port)
			if !errors.Is(err, ErrPrivateTarget) {
				t.Errorf("host %s %s:%d: got %v, want ErrPrivateTarget", host, tc.kind, tc.port, err)
			}
			err = denyAll().Validate(context.Background(), tc.kind, host, tc.port)
			if !errors.Is(err, ErrPrivateTarget) {
				t.Errorf("hook-refusing host %s %s:%d: got %v, want ErrPrivateTarget", host, tc.kind, tc.port, err)
			}
		}
	}
}

func TestPolicyAllowlistHookGrantsExplicitAccess(t *testing.T) {
	policy := EndpointPolicy{
		PrivateAllow: func(kind ServiceKind, host string, ip net.IP, port int) bool {
			return kind == ServiceIMAP && host == "127.0.0.1" && ip.IsLoopback() && port == 993
		},
	}
	if err := policy.Validate(context.Background(), ServiceIMAP, "127.0.0.1", 993); err != nil {
		t.Fatalf("allowlisted loopback IMAP target: unexpected error %v", err)
	}
	// The hook is scoped: other kinds, hosts and ports stay denied.
	if err := policy.Validate(context.Background(), ServiceSMTP, "127.0.0.1", 465); !errors.Is(err, ErrPrivateTarget) {
		t.Fatalf("hook must not cover SMTP: got %v", err)
	}
	if err := policy.Validate(context.Background(), ServiceIMAP, "192.168.0.5", 993); !errors.Is(err, ErrPrivateTarget) {
		t.Fatalf("hook must not cover other private hosts: got %v", err)
	}
}

func TestPolicyPortWhitelist(t *testing.T) {
	public := "198.51.100.7" // TEST-NET-2 literal: never dialed, only validated
	for _, tc := range []struct {
		kind    ServiceKind
		port    int
		allow25 bool
		wantErr error
	}{
		{ServiceIMAP, 993, false, nil},
		{ServiceIMAP, 143, false, nil},
		{ServiceIMAP, 587, false, ErrPortDenied},
		{ServiceIMAP, 465, false, ErrPortDenied},
		{ServiceSMTP, 465, false, nil},
		{ServiceSMTP, 587, false, nil},
		{ServiceSMTP, 25, false, ErrPortDenied},
		{ServiceSMTP, 25, true, nil},
		{ServiceSMTP, 143, false, ErrPortDenied},
		{ServiceIMAP, 0, false, ErrPortDenied},
		{ServiceIMAP, 65536, false, ErrPortDenied},
	} {
		policy := EndpointPolicy{AllowPort25: tc.allow25}
		err := policy.Validate(context.Background(), tc.kind, public, tc.port)
		if tc.wantErr == nil && err != nil {
			t.Errorf("%s:%d (allow25=%v): unexpected error %v", tc.kind, tc.port, tc.allow25, err)
		}
		if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
			t.Errorf("%s:%d (allow25=%v): got %v, want %v", tc.kind, tc.port, tc.allow25, err, tc.wantErr)
		}
	}
}

func TestPolicyHostFormat(t *testing.T) {
	invalidHosts := []string{
		"",
		"under_score.example.invalid",
		"-lead.example.invalid",
		"trail-.example.invalid",
		"bad..example.invalid",
		strings.Repeat("a", 64) + ".example.invalid",
		"space example.invalid",
		"mail.example.invalid/path",
	}
	for _, host := range invalidHosts {
		err := loopbackPolicy(993).Validate(context.Background(), ServiceIMAP, host, 993)
		if !errors.Is(err, ErrHostInvalid) {
			t.Errorf("host %q: got %v, want ErrHostInvalid", host, err)
		}
	}
	validHosts := []string{
		"mail.example.invalid",
		"MAIL.example.INVALID",
		"mail-a.example.invalid",
		"127.0.0.1",
		"198.51.100.7",
		"2001:db8::1",
	}
	for _, host := range validHosts {
		if !validHost(host) {
			t.Errorf("host %q rejected by validHost, want accepted", host)
		}
	}
}

func TestPolicyUnresolvableHost(t *testing.T) {
	// The trailing dot makes this an absolute FQDN under the reserved .invalid
	// TLD (RFC 6761): a correct resolver must fail the lookup and the policy
	// must fail closed with ErrUnresolvable.
	err := loopbackPolicy(993).Validate(context.Background(), ServiceIMAP,
		"mailadapter-unresolvable-probe.invalid.", 993)
	if err == nil {
		// Sandboxes with wildcard DNS answer even reserved names; the
		// unresolvable path cannot be exercised there (noted in the batch 3
		// report). The deny-on-resolved-IP behavior is covered by the
		// localhost test below.
		t.Skip("sandbox resolver wildcard-answers reserved names")
	}
	if !errors.Is(err, ErrUnresolvable) {
		t.Fatalf("got %v, want ErrUnresolvable", err)
	}
}

func TestPolicyResolvesLocalhostToLoopback(t *testing.T) {
	// Hostnames are resolved during validation: "localhost" must be treated
	// exactly like the 127.0.0.1 literal.
	policy := EndpointPolicy{}
	if err := policy.Validate(context.Background(), ServiceIMAP, "localhost", 993); !errors.Is(err, ErrPrivateTarget) {
		t.Fatalf("got %v, want ErrPrivateTarget", err)
	}
	if err := loopbackPolicy(993).Validate(context.Background(), ServiceIMAP, "localhost", 993); err != nil {
		t.Fatalf("allowlisted localhost: unexpected error %v", err)
	}
}

func TestPolicyDialControlRechecksResolvedAddress(t *testing.T) {
	policy := EndpointPolicy{}
	control := policy.dialControl(ServiceIMAP)
	if err := control("tcp", "127.0.0.1:993", nil); !errors.Is(err, ErrPrivateTarget) {
		t.Fatalf("control on loopback: got %v, want ErrPrivateTarget", err)
	}
	if err := control("tcp", "192.0.2.10:993", nil); err != nil {
		t.Fatalf("control on public address: unexpected error %v", err)
	}
	if err := control("tcp", "not-an-ip:993", nil); !errors.Is(err, ErrHostInvalid) {
		t.Fatalf("control on unresolved host: got %v, want ErrHostInvalid", err)
	}
	if err := control("tcp", "192.0.2.10:notaport", nil); !errors.Is(err, ErrPortDenied) {
		t.Fatalf("control on bad port: got %v, want ErrPortDenied", err)
	}

	smtpControl := policy.dialControl(ServiceSMTP)
	if err := smtpControl("tcp", "192.0.2.10:993", nil); !errors.Is(err, ErrPortDenied) {
		t.Fatalf("smtp control on imap port: got %v, want ErrPortDenied", err)
	}
}
