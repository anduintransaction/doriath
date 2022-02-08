package buildtree

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/anduintransaction/doriath/utils"
	"github.com/palantir/stacktrace"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BuildTreeTestSuite struct {
	suite.Suite
	resourceFolder string
}

func (s *BuildTreeTestSuite) SetupTest() {
	s.resourceFolder = "../test-resources"
}

func (s *BuildTreeTestSuite) TestReadConfigfile() {
	fileContent := `
root_dir: .
build:
  - name: "ubuntu"
    tag: "{{.ubuntuTag}}"
    from: "provided"
  - name: "human/aragorn"
    tag: "{{.aragornTag}}"
    from: "./human/aragorn"
    depend: "ubuntu"
    pre_build: "./init.sh"
    post_build: "./finalize.sh"
    force_build: true
    push_latest: true
    platforms:
      - linux/amd64
      - linux/arm64
credentials:
  - name: gcr.io
    registry: "https://gcr.io/v2/"
    username: "username"
    password: "${TEST_PASSWORD}"
  - name: dockerhub
    username: "username"
    password_file: ${TEST_PASSWORD_FILE}
`
	os.Setenv("TEST_PASSWORD", "testpassword")
	os.Setenv("TEST_PASSWORD_FILE", filepath.Join(s.resourceFolder, "credentials", "password"))
	variables := map[string]string{
		"aragornTag": "3.1.4",
	}
	variableFiles := []string{
		filepath.Join(s.resourceFolder, "vars", "vars"),
	}
	buildConfig, err := readBuildConfig([]byte(fileContent), variables, variableFiles)
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
			PushLatest: true,
			Platforms:  []string{"linux/amd64", "linux/arm64"},
		},
	}
	require.Equal(s.T(), expectedBuilds, buildConfig.Build)
	for i := range buildConfig.Credentials {
		resolvedCredential, err := resolveCredential(buildConfig.Credentials[i], buildConfig.RootDir)
		require.Nil(s.T(), err)
		buildConfig.Credentials[i] = resolvedCredential
	}
	expectedCredentials := []*credentialConfig{
		&credentialConfig{
			Name:     "gcr.io",
			Registry: "https://gcr.io/v2/",
			Username: "username",
			Password: "testpassword",
		},
		&credentialConfig{
			Name:         "dockerhub",
			Username:     "username",
			Password:     "rivendell",
			PasswordFile: filepath.Join(s.resourceFolder, "credentials", "password"),
		},
	}
	require.Equal(s.T(), expectedCredentials, buildConfig.Credentials)
}

func (s *BuildTreeTestSuite) TestBuildTreeHappyPath() {
	if !checkDockerhubTestEnable(s.Suite) {
		s.T().Log("Skipping test happy path")
		return
	}
	rootFolder := filepath.Join(s.resourceFolder, "happy-path")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
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
		tag:       "should-not-exist",
		depend:    "library/ubuntu",
		children:  []string{"library/redis"},
		dirty:     true,
	}
	require.Equal(s.T(), expectedChildNode2, s.convertNodeToTestData(buildTree.allNodes["library/nginx"]))
	expectedGrandChildNode2 := &buildNodeForTestData{
		buildRoot: filepath.Join(rootFolder, "grandchild2"),
		name:      "library/redis",
		tag:       "should-not-exist",
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

func (s *BuildTreeTestSuite) TestCyclicCheck() {
	rootFolder := filepath.Join(s.resourceFolder, "cyclic-check")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	_, ok := stacktrace.RootCause(err).(ErrCyclicDependency)
	require.True(s.T(), ok)
}

func (s *BuildTreeTestSuite) TestMismatchImage() {
	rootFolder := filepath.Join(s.resourceFolder, "mismatch-image")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	_, ok := stacktrace.RootCause(err).(ErrMismatchDependencyImage)
	require.True(s.T(), ok)
}

func (s *BuildTreeTestSuite) TestMismatchTag() {
	rootFolder := filepath.Join(s.resourceFolder, "mismatch-tag")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	_, ok := stacktrace.RootCause(err).(ErrMismatchDependencyTag)
	require.True(s.T(), ok)
}

func (s *BuildTreeTestSuite) TestMissingProvidedImage() {
	if !checkDockerhubTestEnable(s.Suite) {
		s.T().Log("Skipping test missing provided image")
		return
	}
	rootFolder := filepath.Join(s.resourceFolder, "missing-provided-image")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	_, ok := stacktrace.RootCause(err).(ErrMissingTag)
	require.True(s.T(), ok)
}

func (s *BuildTreeTestSuite) TestOutdateTag() {
	if !checkDockerhubTestEnable(s.Suite) {
		s.T().Log("Skipping test outdate tag")
		return
	}
	rootFolder := filepath.Join(s.resourceFolder, "outdate-tag")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	_, ok := stacktrace.RootCause(err).(ErrImageTagOutdated)
	require.True(s.T(), ok)
}

func (s *BuildTreeTestSuite) TestPrePostBuild() {
	if !checkDockerTestEnable() {
		s.T().Log("Skipping testing pre-post build")
		return
	}
	rootFolder := filepath.Join(s.resourceFolder, "pre-and-post-build")
	buildTree, err := ReadBuildTreeFromFile(filepath.Join(rootFolder, "doriath.yml"), map[string]string{}, nil)
	require.Nil(s.T(), err, "build tree must be readable")
	err = buildTree.Prepare()
	require.Nil(s.T(), err, "build tree must be able to be prepared")
	err = buildTree.Build()
	require.Nil(s.T(), err, "build tree must be able to be built")
	cmd := exec.Command("docker", "run", "--rm", "node1:1.0")
	output, err := cmd.Output()
	require.Nil(s.T(), err, "docker must run successfully")
	require.Equal(s.T(), "42\n", string(output))
	utils.RunShellCommand("docker rmi node1:1.0")
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
