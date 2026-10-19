package version

const (
	AppName        = "temporal-resource-operator"
	testingVersion = "v0-testing"
)

// These variables get populated at build time...
var (
	GitVersion string
	GitCommit  string
	BuildDate  string
)

// UserAgent returns the HTTP User-Agent header value that the operator
// attaches to all API requests to Temporal Cloud.
func UserAgent() string {
	version := GitVersion
	if version == "" {
		version = testingVersion
	}
	return AppName + "/" + version
}
