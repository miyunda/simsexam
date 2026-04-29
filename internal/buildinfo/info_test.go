package buildinfo

import "testing"

func TestSummary(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
	})

	Version = "v0.1.0"
	Commit = "abc1234"
	BuildTime = "2026-04-24T08:00:00Z"

	got := Summary()
	want := "simsexam v0.1.0 (commit abc1234, built 2026-04-24T08:00:00Z)"
	if got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}

func TestFooterSummaryWithCommit(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
	})

	Version = "v0.1.5"
	Commit = "abcdef123456"

	got := FooterSummary()
	want := "Version v0.1.5 · abcdef1"
	if got != want {
		t.Fatalf("FooterSummary() = %q, want %q", got, want)
	}
}

func TestFooterSummaryWithoutCommit(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
	})

	Version = "dev"
	Commit = "unknown"

	got := FooterSummary()
	want := "Version dev"
	if got != want {
		t.Fatalf("FooterSummary() = %q, want %q", got, want)
	}
}
