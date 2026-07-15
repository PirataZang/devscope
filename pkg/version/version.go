package version

var (
	Version   = "0.1.0"
	Commit    = "dev"
	BuildDate = "unknown"
)

func String() string {
	return Version + " (" + Commit + ") " + BuildDate
}
