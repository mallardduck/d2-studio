package main

import "testing"

func TestQuitIsRefusedAndFrontendAskedUntilConfirmed(t *testing.T) {
	asked, quit := 0, 0
	s := &AppService{emit: func() { asked++ }, quit: func() { quit++ }}

	// Every attempt (window close, Cmd+Q, menu) is refused and forwarded.
	for i := 1; i <= 3; i++ {
		if s.shouldQuit() {
			t.Fatalf("attempt %d: quit allowed before ConfirmQuit", i)
		}
		if asked != i {
			t.Fatalf("attempt %d: frontend asked %d times", i, asked)
		}
	}
	if quit != 0 {
		t.Fatal("application quit without confirmation")
	}

	s.ConfirmQuit()
	if quit != 1 {
		t.Fatalf("ConfirmQuit should quit once, got %d", quit)
	}
	// The quit triggered by ConfirmQuit re-enters shouldQuit and must pass,
	// without asking the frontend again.
	before := asked
	if !s.shouldQuit() {
		t.Fatal("quit refused after ConfirmQuit")
	}
	if asked != before {
		t.Fatal("frontend was asked again after confirming")
	}
}
