// Package mailadapter isolates the IMAP, SMTP and MIME protocol libraries
// (go-imap/v2, go-smtp, go-message) behind a small adapter surface so the
// protocol dependencies never leak into HTTP handlers or cmd entry points.
//
// This is the batch 3 mail adapter skeleton (FORMAL-UI-MAIL-PLAN §3.1). It is
// deliberately minimal: no automatic reconnection, no background polling, no
// business state. The sync layer (internal/mailsync) is the only protocol
// consumer; internal/httpapi touches this package solely to hold the
// EndpointPolicy value it hands to the sync layer, and cmd/ never imports it.
//
// Safety rules enforced here (RFC-0003 §4/§5):
//
//   - Every connection passes an EndpointPolicy pre-flight check (port
//     whitelist, host format, private/loopback denial with an explicit
//     allowlist hook) and a connect-time Control hook that re-validates the
//     resolved IP address, so a DNS change between validation and dial cannot
//     bypass the policy.
//   - Only encrypted TLS modes are accepted; certificate and hostname
//     verification is always on and there is no option to skip it.
//   - Fetched message bytes are untrusted input; MIME parsing reports
//     sentinel errors plus degraded fields instead of panicking.
//   - The adapter never retries a submission and never generates
//     Message-IDs; a submission whose acceptance cannot be ruled out fails
//     with ErrSubmitUnknown and the caller must treat it as "result unknown".
package mailadapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// defaultTimeout bounds a whole IMAP session or SMTP submission when the
// caller configures no explicit timeout or context deadline.
const defaultTimeout = 60 * time.Second

// ServiceKind selects which protocol a policy check applies to, because the
// two services have different port whitelists.
type ServiceKind string

const (
	ServiceIMAP ServiceKind = "imap"
	ServiceSMTP ServiceKind = "smtp"
)

// Endpoint policy sentinels; callers classify denials with errors.Is.
var (
	ErrHostInvalid   = errors.New("mail endpoint host invalid")
	ErrPortDenied    = errors.New("mail endpoint port denied by policy")
	ErrPrivateTarget = errors.New("mail endpoint private or loopback target denied by policy")
	ErrUnresolvable  = errors.New("mail endpoint host unresolvable")
)

// Whitelisted ports per RFC-0003 §5: IMAPS and STARTTLS-capable IMAP for
// receiving, implicit-TLS and standard submission ports for sending. SMTP
// port 25 is legacy MX relaying, not submission, and stays disabled unless
// the deployment explicitly re-enables it.
const (
	imapPortImplicitTLS = 993
	imapPortStartTLS    = 143
	smtpPortImplicitTLS = 465
	smtpPortSubmission  = 587
	smtpPortLegacy      = 25
)

// PrivateAllowFunc reports whether a private or loopback target that is
// denied by default is explicitly permitted by the server administrator
// (plan §3.1: 私网邮件主机仅通过服务器管理员显式允许列表接入).
type PrivateAllowFunc func(kind ServiceKind, host string, ip net.IP, port int) bool

// EndpointPolicy restricts where the adapter may connect. The zero value is
// the strict default: only public hosts on the whitelisted ports.
type EndpointPolicy struct {
	// AllowPort25 re-enables SMTP port 25. It is denied by default because
	// RFC 8314 requires message submission over 465/587.
	AllowPort25 bool
	// PrivateAllow is the administrator allowlist hook. When it returns true
	// for an exact target (service kind, configured host, resolved IP, port)
	// that target is allowed even though it is private/loopback or outside
	// the port whitelist; when it returns false — or is nil — the default
	// rules apply unchanged. Production deployments for public mail hosts
	// leave it nil. The in-process controlled endpoints (127.0.0.1 on random
	// ports) used by this package's tests are the other intended user.
	PrivateAllow PrivateAllowFunc
}

func allowedPort(kind ServiceKind, port int, allow25 bool) bool {
	switch kind {
	case ServiceIMAP:
		return port == imapPortImplicitTLS || port == imapPortStartTLS
	case ServiceSMTP:
		if port == smtpPortImplicitTLS || port == smtpPortSubmission {
			return true
		}
		return allow25 && port == smtpPortLegacy
	default:
		return false
	}
}

// isRestrictedIP reports whether ip points into a range that must not be
// contacted without the explicit allowlist: private, loopback, link-local,
// multicast and unspecified addresses, including their IPv4-mapped IPv6
// forms (Go's helpers normalize through To4).
func isRestrictedIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// checkIP applies the policy to one resolved target. The allowlist hook, if
// present, authorizes one exact endpoint (service kind, host, resolved IP,
// port) — it is the administrator's explicit exception mechanism and may
// therefore also cover a port outside the whitelist. Without a hook, only
// whitelisted ports on non-restricted addresses pass.
func (p EndpointPolicy) checkIP(kind ServiceKind, host string, ip net.IP, port int) error {
	if p.PrivateAllow != nil && p.PrivateAllow(kind, host, ip, port) {
		return nil
	}
	if !allowedPort(kind, port, p.AllowPort25) {
		return fmt.Errorf("%w: %d", ErrPortDenied, port)
	}
	if isRestrictedIP(ip) {
		return fmt.Errorf("%w: %s (%s)", ErrPrivateTarget, ip, kind)
	}
	return nil
}

// Validate runs the pre-flight endpoint checks: port range, host format,
// then resolution of every address the name points at, each target judged by
// the same rules as the connect-time Control hook (allowlist hook first,
// then port whitelist, then restricted-address denial). It must be called
// before any dial.
func (p EndpointPolicy) Validate(ctx context.Context, kind ServiceKind, host string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: %d", ErrPortDenied, port)
	}
	if !validHost(host) {
		return fmt.Errorf("%w: %q", ErrHostInvalid, host)
	}
	ips, err := resolveHost(ctx, host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if err := p.checkIP(kind, host, ip, port); err != nil {
			return err
		}
	}
	return nil
}

func resolveHost(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrUnresolvable, host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvable, host)
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ips = append(ips, addr.IP)
	}
	return ips, nil
}

// validHost accepts RFC 1123 hostnames and IP literals only. Bracketed IPv6
// literals are rejected: the adapter stores bare hosts and joins the port
// itself.
func validHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	if len(labels) == 0 {
		return false
	}
	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			case c == '-' && i > 0 && i < len(label)-1:
			default:
				return false
			}
		}
	}
	return true
}

// dialer builds a *net.Dialer carrying the connect-time Control hook.
func (p EndpointPolicy) dialer(kind ServiceKind, timeout time.Duration, deadline time.Time) *net.Dialer {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	d := &net.Dialer{Timeout: timeout, Control: p.dialControl(kind)}
	if !deadline.IsZero() {
		d.Deadline = deadline
	}
	return d
}

// dialControl returns a net.Dialer Control hook that re-validates the exact
// resolved address at connect time. This closes the gap between the
// pre-flight resolution in Validate and the actual dial (DNS rebinding).
func (p EndpointPolicy) dialControl(kind ServiceKind) func(network, address string, conn syscall.RawConn) error {
	return func(_ string, address string, _ syscall.RawConn) error {
		host, portStr, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrHostInvalid, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("%w: %q", ErrHostInvalid, host)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: %q", ErrPortDenied, portStr)
		}
		return p.checkIP(kind, host, ip, port)
	}
}

// tlsConfig builds the strict TLS configuration shared by both protocols.
// Certificate and hostname verification is always enabled; RootCAs only
// swaps the trust store (for private CAs), it never skips verification
// (RFC-0003 §5: 不允许跳过验证的选项).
func tlsConfig(serverName string, roots *x509.CertPool) *tls.Config {
	return &tls.Config{
		ServerName:         serverName,
		RootCAs:            roots,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}
}

// sessionDeadline returns the absolute deadline for a whole session: the
// earlier of the context deadline and the configured timeout.
func sessionDeadline(ctx context.Context, timeout time.Duration) time.Time {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

// ctxDeadline returns the context deadline, or the zero time.
func ctxDeadline(ctx context.Context) time.Time {
	deadline, _ := ctx.Deadline()
	return deadline
}
