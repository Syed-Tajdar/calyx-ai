package main

import (
	"fmt"
	"os"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: hmacsig <raw-query>")
		os.Exit(1)
	}
	secret := os.Getenv("RINGBA_WEBHOOK_SECRET")
	if secret == "" {
		secret = "dev_ringba_hmac_secret_change_me"
	}
	fmt.Print(security.ComputeRingbaHMAC(nil, os.Args[1], secret))
}
