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

func FooterSummary() string {
	if Commit != "" && Commit != "unknown" {
		return fmt.Sprintf("Version %s · %s", Version, shortCommit(Commit))
	}
	return fmt.Sprintf("Version %s", Version)
}

func shortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}
