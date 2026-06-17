package matcher

import (
	"strings"

	"github.com/Syed-Tajdar/calyx-ai/services/ad-attribution/internal/models"
	"hash/fnv"
)

// ResolvePlatform maps click_id / hints to an ad platform.
func ResolvePlatform(evt models.ConversionEvent) string {
	if hint := strings.ToLower(strings.TrimSpace(evt.PlatformHint)); hint != "" {
		return normalizePlatform(hint)
	}
	click := strings.ToLower(strings.TrimSpace(evt.ClickID))
	switch {
	case strings.Contains(click, "fbclid"), strings.HasPrefix(click, "fb_"), strings.Contains(click, "meta"):
		return "meta"
	case strings.Contains(click, "gclid"), strings.HasPrefix(click, "g_"), strings.Contains(click, "google"):
		return "google"
	case strings.Contains(click, "ttclid"), strings.HasPrefix(click, "tt_"), strings.Contains(click, "tiktok"):
		return "tiktok"
	}
	if evt.CallID == "" {
		return "mock"
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(evt.CallID))
	switch h.Sum32() % 3 {
	case 0:
		return "meta"
	case 1:
		return "google"
	default:
		return "tiktok"
	}
}

func normalizePlatform(name string) string {
	switch name {
	case "meta", "meta_capi", "facebook":
		return "meta"
	case "google", "google_ads":
		return "google"
	case "tiktok":
		return "tiktok"
	case "mock":
		return "mock"
	default:
		return name
	}
}
