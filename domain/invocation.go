package domain

// InvocationRequest is a frontend-originated command call after the adapter
// has established caller identity at the native boundary.
type InvocationRequest struct {
	Caller       Caller
	Command      CommandName
	Input        any
	ResourcePath string // optional; used when the command's permission is path-scoped
}

// InvocationResult is a typed success or structured denial/failure.
type InvocationResult struct {
	Command    CommandName
	Output     any
	Decision   Decision
	Authorized bool
}
