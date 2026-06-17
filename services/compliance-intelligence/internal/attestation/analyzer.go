package attestation

import "hash/fnv"

// Level returns a mock STIR/SHAKEN attestation level from call id hash.
func Level(callID string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(callID))
	switch h.Sum32() % 3 {
	case 0:
		return "A"
	case 1:
		return "B"
	default:
		return "C"
	}
}

// CarrierBlockEstimate estimates revenue at risk from low attestation.
func CarrierBlockEstimate(level string, revenue float32) float32 {
	switch level {
	case "C":
		return revenue * 0.35
	case "B":
		return revenue * 0.10
	default:
		return 0
	}
}
