package llm

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	appID := strings.TrimSpace(os.Getenv("QWEN_APP_ID"))
	apiKey := strings.TrimSpace(os.Getenv("QWEN_API_KEY"))
	if appID == "" || apiKey == "" {
		fmt.Println("skip llm integration tests: QWEN_APP_ID or QWEN_API_KEY not set")
		os.Exit(0)
	}
	os.Exit(m.Run())
}
