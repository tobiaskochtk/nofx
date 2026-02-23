//go:build coinank_integration
// +build coinank_integration

package coinank

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

var TestApikey = strings.TrimSpace(os.Getenv("COINANK_API_KEY"))

func TestMain(m *testing.M) {
	if TestApikey == "" {
		fmt.Println("skip coinank integration tests: COINANK_API_KEY not set")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
