package home

// Service holds the home/dashboard business logic.
type Service struct{}

// NewService builds the home service.
func NewService() *Service { return &Service{} }

// LandingTarget returns where an anonymous visitor should go: the dashboard
// when a user is present, otherwise the empty target (render the landing page).
func (s *Service) LandingTarget(userPresent bool) LandingTarget {
	if userPresent {
		return LandingTargetDashboard
	}
	return LandingTargetNone
}
