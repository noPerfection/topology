package context

import (
	"testing"
	"time"

	"github.com/sds-framework/log-lib"
	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing orchestra
type TestDevCtxSuite struct {
	suite.Suite

	currentDir string // executable to store the binaries and source codes
	url        string // dependency source code
	id         string // the id of the dependency
	ctx        *Context
	logger     *log.Logger
}

// Make sure that Account is set to five
// before each test
func (test *TestDevCtxSuite) SetupTest() {
	s := test.Require

	logger, err := log.New("test", false)
	s().NoError(err)

	test.logger = logger
}

func (test *TestDevCtxSuite) TearDownTest() {
}

// Test_10_New new service by flag or environment variable
func (test *TestDevCtxSuite) Test_10_New() {
	s := test.Suite.Require

	for i := 0; i < 3; i++ {
		test.logger.Info("new context", "i", i)
		ctx, err := New()
		s().NoError(err)
		test.logger.Info("start context", "i", i)
		s().NoError(ctx.StartDepManager())
		time.Sleep(time.Millisecond * 100)
		test.logger.Info("close context", "i", i)
		s().NoError(ctx.Close())
		time.Sleep(time.Millisecond * 100)
	}

	test.logger.Info("context started and closed several times")
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDevCtx(t *testing.T) {
	suite.Run(t, new(TestDevCtxSuite))
}
