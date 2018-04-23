package buildtree

import (
	"path/filepath"
	"testing"

	"github.com/anduintransaction/doriath/utils"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IntegTestSuite struct {
	suite.Suite
}

func (s *IntegTestSuite) TestBuildPush() {
	if !checkDockerhubTestEnable(s.Suite) {
		s.T().Log("Skipping integ test")
		return
	}
	if !checkDockerTestEnable() {
		s.T().Log("Skipping integ test")
		return
	}
	rootFolder := filepath.Join("../test-resources", "real")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	require.Nil(s.T(), err, "build tree must be able to be prepared")
	err = buildTree.Push()
	require.Nil(s.T(), err, "build tree must be able to be built")
	utils.RunShellCommand("docker rmi anduin/doriath-test:1.1")
	utils.RunShellCommand("docker rmi anduin/doriath-test:latest")
}

func TestInteg(t *testing.T) {
	suite.Run(t, new(IntegTestSuite))
}
