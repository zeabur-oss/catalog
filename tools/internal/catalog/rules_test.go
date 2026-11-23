package catalog

import (
	"regexp"
	"testing"
)

func TestTimeoutPatternNeedsAUnitAndAboveZero(t *testing.T) {
	re := regexp.MustCompile(TimeoutPattern)
	for s, want := range map[string]bool{"180s": true, "5m": true, "0s": false, "00m": false, "10": false, "1h": false} {
		if re.MatchString(s) != want {
			t.Errorf("%q: match = %v, want %v", s, !want, want)
		}
	}
}
