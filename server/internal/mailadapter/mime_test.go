package mailadapter

import (
	"errors"
	"strings"
	"testing"
)

func TestParseMIMEMultipartExtractsFields(t *testing.T) {
	raw := syntheticMessage("parse-0001@example.invalid", "Parsed subject",
		"Plain body line.", "<p>HTML <em>body</em></p>")
	parsed, err := ParseMIME(raw)
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}
	if parsed.Subject != "Parsed subject" {
		t.Fatalf("subject %q", parsed.Subject)
	}
	if len(parsed.From) != 1 || parsed.From[0].Address != "sender@example.invalid" {
		t.Fatalf("From %+v", parsed.From)
	}
	if len(parsed.To) != 2 {
		t.Fatalf("To %+v", parsed.To)
	}
	if len(parsed.Cc) != 1 || parsed.Cc[0].Address != "cc-one@example.invalid" {
		t.Fatalf("Cc %+v", parsed.Cc)
	}
	if parsed.Date.IsZero() || parsed.Date.Year() != 2026 {
		t.Fatalf("date %+v", parsed.Date)
	}
	if strings.TrimSpace(parsed.TextBody) != "Plain body line." {
		t.Fatalf("text body %q", parsed.TextBody)
	}
	if !strings.Contains(parsed.HTMLBody, "<em>body</em>") {
		t.Fatalf("html body %q", parsed.HTMLBody)
	}
}

func TestParseMIMEEncodedSubjectAndSimpleBody(t *testing.T) {
	// Subject carries an RFC 2047 encoded word (UTF-8 "Über batch 3"); the
	// body is a plain non-multipart text/plain without Content-Type.
	raw := []byte("From: sender@example.invalid\r\n" +
		"To: user@example.invalid\r\n" +
		"Subject: =?utf-8?b?w5xiZXIgYmF0Y2ggMw==?=\r\n" + // "Über batch 3"
		"Date: Fri, 06 Nov 2026 09:30:00 +0000\r\n" +
		"Message-ID: <parse-0002@example.invalid>\r\n" +
		"\r\n" +
		"Simple body without MIME structure.\r\n")
	parsed, err := ParseMIME(raw)
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}
	if parsed.Subject != "Über batch 3" {
		t.Fatalf("subject %q", parsed.Subject)
	}
	if strings.TrimSpace(parsed.TextBody) != "Simple body without MIME structure." {
		t.Fatalf("text body %q", parsed.TextBody)
	}
	if parsed.HTMLBody != "" {
		t.Fatalf("unexpected html body %q", parsed.HTMLBody)
	}
}

// TestParseMIMEHTMLVerbatim proves the adapter does not sanitize HTML: the
// script tag survives. Sanitization is enforced by the upper mailstore/HTTP
// layer (plan §2.3), and this test documents the split of responsibilities.
func TestParseMIMEHTMLVerbatim(t *testing.T) {
	html := "<p>x</p><script>alert('untrusted')</script>"
	raw := syntheticMessage("parse-0003@example.invalid", "HTML verbatim", "text", html)
	parsed, err := ParseMIME(raw)
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}
	if !strings.Contains(parsed.HTMLBody, "<script>alert('untrusted')</script>") {
		t.Fatalf("html body was modified: %q", parsed.HTMLBody)
	}
}

func TestParseMIMESkipsAttachments(t *testing.T) {
	raw := []byte("From: sender@example.invalid\r\n" +
		"To: user@example.invalid\r\n" +
		"Subject: attachment skip\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mix\"\r\n" +
		"\r\n" +
		"--mix\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"\r\n" +
		"Visible text.\r\n" +
		"--mix\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=\"payload.bin\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"AAAABBBBCCCC\r\n" +
		"--mix--\r\n")
	parsed, err := ParseMIME(raw)
	if err != nil {
		t.Fatalf("ParseMIME: %v", err)
	}
	if strings.TrimSpace(parsed.TextBody) != "Visible text." {
		t.Fatalf("text body %q", parsed.TextBody)
	}
	if strings.Contains(parsed.TextBody, "AAAABBBB") {
		t.Fatalf("attachment content leaked into body: %q", parsed.TextBody)
	}
}

func TestParseMIMETruncatedHeaderIsErrorWithoutPanic(t *testing.T) {
	raw := syntheticMessage("parse-0004@example.invalid", "Truncated", "text", "<p>h</p>")
	truncated := raw[:40] // cut inside the header block
	parsed, err := ParseMIME(truncated)
	if !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("got %v, want ErrMalformedMIME", err)
	}
	if parsed.Subject != "" || parsed.TextBody != "" {
		t.Fatalf("expected no usable fields, got %+v", parsed)
	}
}

func TestParseMIMETruncatedMultipartIsDegraded(t *testing.T) {
	raw := syntheticMessage("parse-0005@example.invalid", "Truncated body", "Usable plain text.", "<p>cut</p>")
	// Drop the closing boundary: the text part stays complete, the message
	// stays structurally incomplete.
	cut := strings.Index(string(raw), "--batch3-alt--")
	if cut < 0 {
		t.Fatal("marker not found")
	}
	truncated := raw[:cut]
	parsed, err := ParseMIME(truncated)
	if !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("got %v, want ErrMalformedMIME", err)
	}
	if parsed.Subject != "Truncated body" {
		t.Fatalf("degraded subject %q, want the intact header", parsed.Subject)
	}
	if strings.TrimSpace(parsed.TextBody) != "Usable plain text." {
		t.Fatalf("degraded text body %q", parsed.TextBody)
	}
}

func TestParseMIMEBadTransferEncodingIsErrorWithoutPanic(t *testing.T) {
	raw := []byte("From: sender@example.invalid\r\n" +
		"To: user@example.invalid\r\n" +
		"Subject: bad base64\r\n" +
		"Message-ID: <parse-0006@example.invalid>\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" +
		"!!!not-valid-base64!!!\r\n")
	parsed, err := ParseMIME(raw)
	if !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("got %v, want ErrMalformedMIME", err)
	}
	if parsed.TextBody != "" {
		t.Fatalf("undecodable body must not be returned: %q", parsed.TextBody)
	}
}

func TestParseMIMEUnknownCharsetIsDegraded(t *testing.T) {
	raw := []byte("From: sender@example.invalid\r\n" +
		"To: user@example.invalid\r\n" +
		"Subject: unknown charset\r\n" +
		"Message-ID: <parse-0007@example.invalid>\r\n" +
		"Content-Type: text/plain; charset=x-nonexistent-charset\r\n" +
		"\r\n" +
		"Undecodable but intact bytes.\r\n")
	parsed, err := ParseMIME(raw)
	if !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("got %v, want ErrMalformedMIME", err)
	}
	if parsed.Subject != "unknown charset" {
		t.Fatalf("degraded subject %q", parsed.Subject)
	}
	if parsed.TextBody == "" {
		t.Fatal("raw body bytes should stay usable as degraded output")
	}
}

func TestParseMIMEEmptyAndJunkInput(t *testing.T) {
	if _, err := ParseMIME(nil); !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("nil input: got %v, want ErrMalformedMIME", err)
	}
	if _, err := ParseMIME([]byte{}); !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("empty input: got %v, want ErrMalformedMIME", err)
	}
	junk := make([]byte, 0, 64)
	for i := 0; i < 64; i++ {
		junk = append(junk, byte(i))
	}
	if _, err := ParseMIME(junk); !errors.Is(err, ErrMalformedMIME) {
		t.Fatalf("binary junk: got %v, want ErrMalformedMIME", err)
	}
}
