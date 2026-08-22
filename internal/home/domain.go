package home

// LandingTarget is the redirect target for the landing page.
type LandingTarget string

const (
	// LandingTargetNone means render the landing page (anonymous visitor).
	LandingTargetNone LandingTarget = ""
	// LandingTargetDashboard redirects logged-in users to the dashboard.
	LandingTargetDashboard LandingTarget = "/dashboard"
)
