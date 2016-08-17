package api

import "testing"

func TestMatcher(t *testing.T) {
	if !match("test:*", "test:event") {
		t.Fatal("test:event should match to test:*")
	}

	if match("testing:*", "test:event") {
		t.Fatal("test:event should not match to testing:*")
	}
}
