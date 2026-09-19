package execplan

type ProcessResult struct {
	Started       bool
	ExitCode      int
	Signal        string
	RequestedStop string
	Diagnostic    error
}
