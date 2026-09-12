package argoutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_custom_startup_script_dex(t *testing.T) {
	expectedOutput := fmt.Sprintf(customBootstrapScriptTemplate, "", "cp /tmp/base.yaml /tmp/dex.yaml")
	require.EqualValues(t, expectedOutput, DexServerCustomStartupScript()[0])
}
