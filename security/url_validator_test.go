package security

import "testing"

func TestValidateURL_AllowsTrustedInternalHost(t *testing.T) {
	if err := ValidateURL("http://selfhosted-ai500:8081/health"); err != nil {
		t.Fatalf("expected trusted selfhosted host to be allowed, got: %v", err)
	}
}

func TestValidateURL_BlocksLoopbackHost(t *testing.T) {
	if err := ValidateURL("http://localhost:8081/health"); err == nil {
		t.Fatal("expected localhost to be blocked")
	}
}

func TestValidateURL_BlocksPrivateIPLiteral(t *testing.T) {
	if err := ValidateURL("http://172.20.0.4:8081/health"); err == nil {
		t.Fatal("expected private IP literal to be blocked")
	}
}
