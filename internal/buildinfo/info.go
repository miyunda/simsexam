package buildinfo

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func Summary() string {
	return fmt.Sprintf("simsexam %s (commit %s, built %s)", Version, Commit, BuildTime)
}
