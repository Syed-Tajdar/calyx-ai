package connectors

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func firstValue(values url.Values, keys ...string) string {
	for _, key := range keys {
		if v := values.Get(key); v != "" {
			return v
		}
	}
	return ""
}

func parseFloat(values url.Values, keys ...string) (float64, error) {
	s := firstValue(values, keys...)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float %q: %w", s, err)
	}
	return f, nil
}

func parseBool(values url.Values, keys ...string) bool {
	s := firstValue(values, keys...)
	return strings.EqualFold(s, "true") || s == "1" || strings.EqualFold(s, "yes")
}

func parseTags(values url.Values) map[string]string {
	tags := make(map[string]string)
	for key, vals := range values {
		if strings.HasPrefix(key, "tag[") && strings.HasSuffix(key, "]") {
			inner := strings.TrimSuffix(strings.TrimPrefix(key, "tag["), "]")
			if inner != "" && len(vals) > 0 {
				tags[inner] = vals[0]
			}
		}
		if key == "tags" && len(vals) > 0 {
			for _, pair := range strings.Split(vals[0], ",") {
				parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
				if len(parts) == 2 {
					tags[parts[0]] = parts[1]
				}
			}
		}
	}
	return tags
}
