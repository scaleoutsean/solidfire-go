package main

import (
	"fmt"

	"github.com/scaleoutsean/solidfire-go/sdk"
)

func main() {
	// This example demonstrates how to use the security helpers provided by the SDK
	// to handle sensitive Account CHAP secrets.

	// 1. Create a dummy Account object (similar to what ListAccounts would return)
	account := &sdk.Account{
		AccountID:       123,
		Username:        "example-user",
		Status:          "active",
		InitiatorSecret: "shhhh-secret-initiator-chap",
		TargetSecret:    "shhhh-secret-target-chap",
	}

	fmt.Println("--- Before Redaction ---")

	// 2. Implicit Redaction: Using %v or fmt.Println uses the String() method
	// which automatically hides secrets from logs/output.
	fmt.Printf("Logging the account object (Stringer interface): %v\n", account)

	// Note: Accessing fields directly still works if you actually need them
	fmt.Printf("Direct field access (if needed): InitiatorSecret is %q\n", account.InitiatorSecret)

	fmt.Println("\n--- After Redaction ---")

	// 3. Explicit Redaction: Calling Redact() clears the fields in memory.
	// This is recommended before passing the object to other layers of your app
	// if you no longer need the secrets.
	account.Redact()

	fmt.Printf("Logging after account.Redact(): %v\n", account)
	fmt.Printf("Direct field access after Redact(): InitiatorSecret is %q\n", account.InitiatorSecret)

	// Note on API usage:
	/*
		ctx := context.Background()
		var sf sdk.SFClient
		sf.Connect(ctx, "192.168.1.1", "12.5", "admin", "password")

		res, err := sf.GetAccountByName(ctx, &sdk.GetAccountByNameRequest{Username: "example-user"})
		if err == nil {
			account := &res.Account
			account.Redact() // Clear secrets immediately after fetching if not needed
		}
	*/
}
