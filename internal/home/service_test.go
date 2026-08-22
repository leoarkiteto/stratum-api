package home

import "testing"

func TestLandingTarget(t *testing.T) {
	s := NewService()
	if got := s.LandingTarget(true); got != LandingTargetDashboard {
		t.Errorf("LandingTarget(true) = %q, want %q", got, LandingTargetDashboard)
	}
	if got := s.LandingTarget(false); got != LandingTargetNone {
		t.Errorf("LandingTarget(false) = %q, want %q", got, LandingTargetNone)
	}
}
