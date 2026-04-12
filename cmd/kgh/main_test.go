package main

import "testing"

func TestRun_Help(t *testing.T) {
	if code := run([]string{"help"}); code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	if code := run([]string{"wat"}); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}
