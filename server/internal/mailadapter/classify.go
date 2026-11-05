// Classification helpers for Dial failures. New in the batch 3 sync slice;
// nothing here changes existing adapter behavior. The sync layer needs to
// tell an authentication rejection apart from other connect failures without
// importing the IMAP protocol library, so the check lives beside the only
// code that may know about protocol details.
package mailadapter

import (
	"errors"

	"github.com/emersion/go-imap/v2"
)

// IsAuthFailure reports whether err (as returned by Dial) is a tagged NO
// [AUTHENTICATIONFAILED] status response from the remote IMAP server, i.e.
// the server rejected the supplied credentials. Servers that reject login
// without that response code are reported as false and stay unclassified
// connect failures; this helper never guesses.
func IsAuthFailure(err error) bool {
	var imapErr *imap.Error
	if !errors.As(err, &imapErr) {
		return false
	}
	return imapErr.Type == imap.StatusResponseTypeNo &&
		imapErr.Code == imap.ResponseCodeAuthenticationFailed
}
