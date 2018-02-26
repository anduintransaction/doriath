package buildtree

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
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
    prebuild: "./init.sh"
    postbuild: "./finalize.sh"
    forcebuild: true
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
	require.Nil(s.T(), err, "read build config should be successful")
	require.Equal(s.T(), ".", buildConfig.RootDir)
	expectedBuilds := []*buildNodeConfig{
		&buildNodeConfig{
			Name: "ubuntu",
			Tag:  "16.04",
			From: "provided",
		},
		&buildNodeConfig{
			Name:       "human/aragorn",
			Tag:        "3.1.4",
			From:       "./human/aragorn",
			Depend:     "ubuntu",
			PreBuild:   "./init.sh",
			PostBuild:  "./finalize.sh",
			ForceBuild: true,
		},
	}
	require.Equal(s.T(), expectedBuilds, buildConfig.Build)
	expectedCredentials := []*credentialConfig{
		&credentialConfig{
			Name:     "gcr.io",
			Registry: "https://gcr.io/v2/",
			Username: "username",
			Password: "testpassword",
		},
	}
	require.Equal(s.T(), expectedCredentials, buildConfig.Credentials)
}

func (s *BuildTreeTestSuite) TestBuildTreeHappyPath() {
	if !s.checkIntegTestEnable() {
		s.T().Log("Skipping test happy path")
		return
	}
	rootFolder := "../test-resources/happy-path"
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{})
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	require.Nil(s.T(), err, "build tree should be able to be prepared")
	expectedProvidedNode := &buildNodeForTestData{
		buildRoot: "provided",
		name:      "library/debian",
		tag:       "8",
		depend:    "",
		children:  []string{"library/ubuntu"},
		dirty:     false,
	}
	require.Equal(s.T(), expectedProvidedNode, s.convertNodeToTestData(buildTree.allNodes["library/debian"]))
	expectedParentNode1 := &buildNodeForTestData{
		buildRoot: filepath.Join(rootFolder, "parent1"),
		name:      "library/ubuntu",
		tag:       "16.04",
		depend:    "library/debian",
		children:  []string{"library/alpine", "library/nginx", "library/postgres"},
		dirty:     false,
	}
	require.Equal(s.T(), expectedParentNode1, s.convertNodeToTestData(buildTree.allNodes["library/ubuntu"]))
	expectedChildNode1 := &buildNodeForTestData{
		buildRoot: filepath.Join(rootFolder, "child1"),
		name:      "library/alpine",
		tag:       "3.5",
		depend:    "library/ubuntu",
		children:  []string{"library/busybox"},
		dirty:     false,
	}
	require.Equal(s.T(), expectedChildNode1, s.convertNodeToTestData(buildTree.allNodes["library/alpine"]))
	expectedGrandChildNode1 := &buildNodeForTestData{
		buildRoot: filepath.Join(rootFolder, "grandchild1"),
		name:      "library/busybox",
		tag:       "1",
		depend:    "library/alpine",
		children:  []string{},
		dirty:     false,
	}
	require.Equal(s.T(), expectedGrandChildNode1, s.convertNodeToTestData(buildTree.allNodes["library/busybox"]))
	expectedChildNode2 := &buildNodeForTestData{
		buildRoot: filepath.Join(rootFolder, "child2"),
		name:      "library/nginx",
		tag:       "should-not-exists",
		depend:    "library/ubuntu",
		children:  []string{"library/redis"},
		dirty:     true,
	}
	require.Equal(s.T(), expectedChildNode2, s.convertNodeToTestData(buildTree.allNodes["library/nginx"]))
	expectedGrandChildNode2 := &buildNodeForTestData{
		buildRoot: filepath.Join(rootFolder, "grandchild2"),
		name:      "library/redis",
		tag:       "should-not-exists",
		depend:    "library/nginx",
		children:  []string{},
		dirty:     true,
	}
	require.Equal(s.T(), expectedGrandChildNode2, s.convertNodeToTestData(buildTree.allNodes["library/redis"]))
	expectedChildNode3 := &buildNodeForTestData{
		buildRoot:  filepath.Join(rootFolder, "child3"),
		name:       "library/postgres",
		tag:        "9.6",
		depend:     "library/ubuntu",
		children:   []string{"library/mariadb"},
		dirty:      true,
		forceBuild: true,
	}
	require.Equal(s.T(), expectedChildNode3, s.convertNodeToTestData(buildTree.allNodes["library/postgres"]))
	expectedGrandChildNode3 := &buildNodeForTestData{
		buildRoot:  filepath.Join(rootFolder, "grandchild3"),
		name:       "library/mariadb",
		tag:        "10",
		depend:     "library/postgres",
		children:   []string{},
		dirty:      true,
		forceBuild: true,
	}
	require.Equal(s.T(), expectedGrandChildNode3, s.convertNodeToTestData(buildTree.allNodes["library/mariadb"]))
}

func (s *BuildTreeTestSuite) checkIntegTestEnable() bool {
	integTestEnv := os.Getenv("INTEG_TEST_ENABLE")
	if integTestEnv == "1" || integTestEnv == "true" {
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

type buildNodeForTestData struct {
	buildRoot  string
	name       string
	tag        string
	depend     string
	children   []string
	dirty      bool
	forceBuild bool
}

func (s *BuildTreeTestSuite) convertNodeToTestData(node *buildNode) *buildNodeForTestData {
	testData := &buildNodeForTestData{
		buildRoot:  node.buildRoot,
		name:       node.name,
		tag:        node.tag,
		depend:     node.depend,
		children:   []string{},
		dirty:      node.dirty,
		forceBuild: node.forceBuild,
	}
	for _, child := range node.children {
		testData.children = append(testData.children, child.name)
	}
	sort.Strings(testData.children)
	return testData
}

func TestBuildTree(t *testing.T) {
	suite.Run(t, new(BuildTreeTestSuite))
}
