package buildtree

import (
	"os"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func checkDockerTestEnable() bool {
	dockerTestEnv := os.Getenv("DOCKER_TEST_ENABLE")
	return dockerTestEnv == "1" || dockerTestEnv == "true"
}

func checkDockerhubTestEnable(s suite.Suite) bool {
	dockerhubTestEnv := os.Getenv("DOCKERHUB_TEST_ENABLE")
	if dockerhubTestEnv == "1" || dockerhubTestEnv == "true" {
		if os.Getenv("DOCKERHUB_USERNAME") == "" {
			require.Fail(s.T(), "DOCKERHUB_USERNAME was not defined")
		}
		if os.Getenv("DOCKERHUB_PASSWORD") == "" {
			require.Fail(s.T(), "DOCKERHUB_PASSWORD was not defined")
		}
		return true
	}
	return false
}
