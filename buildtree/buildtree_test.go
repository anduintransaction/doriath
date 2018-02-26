package buildtree

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BuildTreeTestSuite struct {
	suite.Suite
}

func (s *BuildTreeTestSuite) TestReadConfigfile() {
	fileContent := `
root_dir: .
build:
  - name: "ubuntu"
    tag: "16.04"
    from: "provided"
  - name: "human/aragorn"
    tag: "{{.aragornTag}}"
    from: "./human/aragorn"
    depend: "ubuntu"
credentials:
  - name: gcr.io
    registry: "https://gcr.io/v2/"
    username: "username"
    password: "${TEST_PASSWORD}"
`
	os.Setenv("TEST_PASSWORD", "testpassword")
	variables := map[string]string{
		"aragornTag": "3.1.4",
	}
	buildConfig, err := readBuildConfig([]byte(fileContent), variables)
	assert.Nil(s.T(), err, "read build config should be successful")
	assert.Equal(s.T(), ".", buildConfig.RootDir)
	expectedBuilds := []*buildNodeConfig{
		&buildNodeConfig{
			Name: "ubuntu",
			Tag:  "16.04",
			From: "provided",
		},
		&buildNodeConfig{
			Name:   "human/aragorn",
			Tag:    "3.1.4",
			From:   "./human/aragorn",
			Depend: "ubuntu",
		},
	}
	assert.Equal(s.T(), expectedBuilds, buildConfig.Build)
	expectedCredentials := []*credentialConfig{
		&credentialConfig{
			Name:     "gcr.io",
			Registry: "https://gcr.io/v2/",
			Username: "username",
			Password: "testpassword",
		},
	}
	assert.Equal(s.T(), expectedCredentials, buildConfig.Credentials)
}

func TestBuildTree(t *testing.T) {
	suite.Run(t, new(BuildTreeTestSuite))
}
