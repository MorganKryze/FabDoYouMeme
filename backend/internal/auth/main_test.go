package auth_test

import (
	"os"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupSuite(m))
}
