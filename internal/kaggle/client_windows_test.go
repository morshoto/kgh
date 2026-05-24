//go:build windows

package kaggle

import "testing"

func TestNewClientCompilesOnWindows(t *testing.T) {
	if NewClient() == nil {
		t.Fatal("expected NewClient to return a client")
	}
}
