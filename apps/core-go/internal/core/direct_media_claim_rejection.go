package core

// The public failure code stays closed. Owner diagnostics and the correcting
// model receive the precise predicate and candidate quote that were rejected.
type directMediaClaimRejection struct {
	Reason string
	Quote  string
	Cause  error
}

func (e *directMediaClaimRejection) Error() string {
	return "direct_media_delivery_unverified: " + e.Reason
}
func (e *directMediaClaimRejection) Unwrap() error { return e.Cause }
