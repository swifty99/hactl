//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/swifty99/hactl/internal/hatest"
)

// ha is the shared HA instance for all integration tests in this package.
// Started once in TestMain, reused across all tests.
var ha *hatest.Instance

func TestMain(m *testing.M) {
	var code int
	ha, code = hatest.StartMain(m, hatest.WithFixture("basic"))
	if code != 0 {
		os.Exit(code)
	}
	exitCode := m.Run()
	ha.Stop()
	if faultyHA != nil {
		faultyHA.Stop()
	}
	if realisticHA != nil {
		realisticHA.Stop()
	}
	os.Exit(exitCode)
}
