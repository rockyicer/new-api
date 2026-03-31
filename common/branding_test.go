package common

import "testing"

func TestDefaultBranding(t *testing.T) {
	if SystemName != "JustAPI" {
		t.Fatalf("expected default system name to be JustAPI, got %q", SystemName)
	}

	if Logo != "/justapi_icon.png" {
		t.Fatalf("expected default logo to be /justapi_icon.png, got %q", Logo)
	}
}
