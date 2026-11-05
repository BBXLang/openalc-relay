package mailadapter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

// ErrMalformedMIME reports untrusted message input that could not be parsed
// completely. The returned ParsedMessage may still carry the fields that
// were recoverable (degraded extraction); callers must handle both the
// error and the fields.
var ErrMalformedMIME = errors.New("malformed mail message")

// maxParsedBodyBytes bounds how much of one untrusted MIME body part is
// read into memory.
const maxParsedBodyBytes = 10 << 20

// ParsedMessage is the extracted view of one raw RFC 5322 message.
type ParsedMessage struct {
	Subject string
	From    []*netmail.Address
	To      []*netmail.Address
	Cc      []*netmail.Address
	Date    time.Time
	// TextBody is the first text/plain part after transfer-encoding and
	// charset decoding (UTF-8).
	TextBody string
	// HTMLBody is the first text/html part, returned VERBATIM. It is
	// untrusted input: HTML sanitization (script/event-handler/dangerous-URL
	// stripping, remote image blocking) is the responsibility of the upper
	// mailstore/HTTP layer (plan §2.3/§3.1), never of this adapter. Do not
	// render or persist it as-is.
	HTMLBody string
}

// ParseMIME extracts subject, addressing headers, date and text/HTML bodies
// from one raw message fetched from IMAP. The input is untrusted remote
// data:
//
//   - malformed or truncated input never panics; it returns an error wrapping
//     ErrMalformedMIME together with whatever fields were recoverable;
//   - unsupported charsets or transfer encodings degrade the affected field
//     and are reported through the same error;
//   - attachment parts are not decoded (attachment fetch is a later batch);
//   - HTML is returned unsanitized on purpose (see ParsedMessage.HTMLBody).
func ParseMIME(raw []byte) (parsed ParsedMessage, err error) {
	// Belt-and-braces guarantee for the untrusted-input contract above: a
	// parser panic must surface as an error, never crash the server.
	defer func() {
		if r := recover(); r != nil {
			parsed = ParsedMessage{}
			err = fmt.Errorf("%w: parser panic on untrusted input: %v", ErrMalformedMIME, r)
		}
	}()
	if len(raw) == 0 {
		return ParsedMessage{}, fmt.Errorf("%w: empty message", ErrMalformedMIME)
	}
	reader, createErr := mail.CreateReader(bytes.NewReader(raw))
	if createErr != nil && !message.IsUnknownCharset(createErr) {
		return ParsedMessage{}, fmt.Errorf("%w: %v", ErrMalformedMIME, createErr)
	}
	defer reader.Close()

	// degraded keeps the first problem found while still extracting usable
	// fields; it is reported at the end.
	var degraded error
	degrade := func(err error) {
		if degraded == nil {
			degraded = err
		}
	}
	if createErr != nil {
		degrade(fmt.Errorf("unknown charset or encoding: %v", createErr))
	}

	header := reader.Header
	if subject, err := header.Subject(); err != nil {
		degrade(fmt.Errorf("subject: %v", err))
		parsed.Subject = header.Get("Subject")
	} else {
		parsed.Subject = subject
	}
	parsed.From = parseAddressList(&header, "From", degrade)
	parsed.To = parseAddressList(&header, "To", degrade)
	parsed.Cc = parseAddressList(&header, "Cc", degrade)
	if date, err := header.Date(); err != nil {
		degrade(fmt.Errorf("date: %v", err))
	} else {
		parsed.Date = date
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if message.IsUnknownCharset(err) && part != nil {
				degrade(fmt.Errorf("part charset: %v", err))
			} else {
				degrade(fmt.Errorf("part: %v", err))
				break
			}
		}
		if part == nil {
			break
		}
		contentType, ok := partContentType(part, degrade)
		if !ok {
			continue
		}
		if _, isAttachment := part.Header.(*mail.AttachmentHeader); isAttachment {
			// Attachments are fetched separately in a later batch.
			continue
		}
		body, readErr := readBounded(part.Body)
		if readErr != nil {
			degrade(readErr)
			continue
		}
		switch contentType {
		case "text/plain":
			if parsed.TextBody == "" {
				parsed.TextBody = body
			}
		case "text/html":
			if parsed.HTMLBody == "" {
				parsed.HTMLBody = body
			}
		}
	}

	if boundary := topBoundary(header); boundary != "" &&
		!bytes.Contains(raw, []byte("--"+boundary+"--")) {
		degrade(errors.New("multipart body truncated: closing boundary missing"))
	}

	if degraded != nil {
		return parsed, fmt.Errorf("%w: %v", ErrMalformedMIME, degraded)
	}
	return parsed, nil
}

// partContentType resolves the media type of one part. A missing
// Content-Type defaults to text/plain per RFC 2046; an unparsable one
// degrades and skips the part.
func partContentType(part *mail.Part, degrade func(error)) (string, bool) {
	value := part.Header.Get("Content-Type")
	if value == "" {
		return "text/plain", true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		degrade(fmt.Errorf("content type %q: %v", value, err))
		return "", false
	}
	return strings.ToLower(mediaType), true
}

// parseAddressList parses one addressing header. Unparsable values degrade
// to a plain std-library attempt and, failing that, an empty list.
func parseAddressList(header *mail.Header, key string, degrade func(error)) []*netmail.Address {
	addresses, err := header.AddressList(key)
	if err == nil {
		return addresses
	}
	degrade(fmt.Errorf("%s: %v", strings.ToLower(key), err))
	raw := header.Get(key)
	if raw == "" {
		return nil
	}
	fallback, err := netmail.ParseAddressList(raw)
	if err != nil {
		degrade(fmt.Errorf("%s fallback: %v", strings.ToLower(key), err))
		return nil
	}
	return fallback
}

func readBounded(body io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxParsedBodyBytes+1))
	if err != nil {
		return "", fmt.Errorf("body: %v", err)
	}
	if len(data) > maxParsedBodyBytes {
		return "", fmt.Errorf("body: exceeds %d bytes", maxParsedBodyBytes)
	}
	return string(data), nil
}

// topBoundary returns the boundary parameter of the top-level multipart
// Content-Type, or "" for non-multipart messages.
func topBoundary(header mail.Header) string {
	value := header.Get("Content-Type")
	if value == "" {
		return ""
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return ""
	}
	return params["boundary"]
}
