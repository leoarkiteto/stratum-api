package electiontemplates

import "time"

// fmtDate formats a nullable timestamp for display, or "" when nil.
func fmtDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("Jan 2, 2006")
}
