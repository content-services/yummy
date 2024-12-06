package yum

import (
	_ "embed"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModuleMDs(t *testing.T) {
	f, err := os.Open("mocks/module.yaml.zst")
	assert.NoError(t, err)

	parsed, err := parseModuleMDs(f)
	assert.NoError(t, err)
	assert.Equal(t, 11, len(parsed))
	assert.NotEmpty(t, parsed[0].Data.Name)
	assert.NotEmpty(t, parsed[0].Data.Artifacts.Rpms)
}

func TestParseRhel8Modules(t *testing.T) {
	f, err := os.Open("mocks/rhel8.modules.yaml.gz")
	assert.NoError(t, err)
	defer f.Close()
	require.NoError(t, err)

	modules, err := parseModuleMDs(f)
	require.NoError(t, err)

	assert.Len(t, modules, 862)

	assert.NotEmpty(t, modules)
	found := false
	for _, module := range modules {
		if module.Data.Name == "ruby" && module.Data.Stream == "2.5" {
			found = true
			assert.NotEmpty(t, module.Data.Artifacts.Rpms)
			assert.NotEmpty(t, module.Data.Profiles)
			value, ok := module.Data.Profiles["common"]
			assert.True(t, ok)
			assert.Equal(t, []string{"ruby"}, value.Rpms)
		}
	}
	assert.True(t, found)
}
