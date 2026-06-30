package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Syed-Tajdar/calyx-ai/services/ingestion/internal/security"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: platformsig <platform> <raw-query>")
		os.Exit(1)
	}
	platform := strings.ToLower(os.Args[1])
	rawQuery := os.Args[2]

	switch platform {
	case "ringba":
		secret := envOrDefault("RINGBA_WEBHOOK_SECRET", "dev_ringba_hmac_secret_change_me")
		fmt.Print(security.ComputeRingbaHMAC(nil, rawQuery, secret))
	case "retreaver":
		secret := envOrDefault("RETREAVER_WEBHOOK_SECRET", "dev_retreaver_hmac_secret_change_me")
		fmt.Print(security.ComputeRetreaverHMAC(nil, rawQuery, secret))
	case "callgrid":
		secret := envOrDefault("CALLGRID_WEBHOOK_SECRET", "dev_callgrid_hmac_secret_change_me")
		fmt.Print(security.ComputeCallGridHMAC(nil, rawQuery, secret))
	default:
		fmt.Fprintf(os.Stderr, "unsupported platform %q (use ringba, retreaver, callgrid)\n", platform)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
