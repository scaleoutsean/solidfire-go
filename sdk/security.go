package sdk

import "fmt"

// Redact clears sensitive CHAP secrets from the Account object.
// Useful if you only need the AccountID or Username and want to ensure
// secrets aren't held in memory or passed to other layers.
func (a *Account) Redact() {
	if a == nil {
		return
	}
	a.InitiatorSecret = ""
	a.TargetSecret = ""
}

// String implements the fmt.Stringer interface to prevent secrets
// from being leaked accidentally in logs or prints.
func (a Account) String() string {
	return fmt.Sprintf(
		"Account{AccountID: %d, Username: %q, Status: %q, InitiatorSecret: <REDACTED>, TargetSecret: <REDACTED>}",
		a.AccountID, a.Username, a.Status,
	)
}
