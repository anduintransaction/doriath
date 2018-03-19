package buildtree

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anduintransaction/doriath/utils"
	"github.com/palantir/stacktrace"
	yaml "gopkg.in/yaml.v2"
)

// BuildTree is a build tree
type BuildTree struct {
	rootDir     string
	rootNodes   []*buildNode
	allNodes    map[string]*buildNode
	credentials map[string]*credentialConfig
}

type buildNode struct {
	buildRoot  string
	name       string
	tag        string
	depend     string
	preBuild   string
	postBuild  string
	children   []*buildNode
	dirty      bool
	forceBuild bool
}

type config struct {
	RootDir     string              `yaml:"root_dir"`
	Build       []*buildNodeConfig  `yaml:"build"`
	Credentials []*credentialConfig `yaml:"credentials"`
}

type buildNodeConfig struct {
	Name       string `yaml:"name"`
	From       string `yaml:"from"`
	Tag        string `yaml:"tag"`
	Depend     string `yaml:"depend"`
	PreBuild   string `yaml:"prebuild"`
	PostBuild  string `yaml:"postbuild"`
	ForceBuild bool   `yaml:"forcebuild"`
}

type credentialConfig struct {
	Name     string `yaml:"name"`
	Registry string `yaml:"registry"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ReadBuildTree reads a build tree from reader
func ReadBuildTree(r io.Reader, variableMap map[string]string) (*BuildTree, error) {
	fileContent, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot read build content")
	}
	return readBuildTree("", fileContent, variableMap)
}

// ReadBuildTreeFromFile reads BuildTree from a build file
func ReadBuildTreeFromFile(buildFile string, variableMap map[string]string) (*BuildTree, error) {
	fileContent, err := ioutil.ReadFile(buildFile)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot read build file %q", buildFile)
	}
	return readBuildTree(buildFile, fileContent, variableMap)
}

func readBuildTree(configFilePath string, fileContent []byte, variableMap map[string]string) (*BuildTree, error) {
	buildConfig, err := readBuildConfig(fileContent, variableMap)
	if err != nil {
		return nil, err
	}
	configFileFolder := filepath.Dir(configFilePath)
	buildTree := &BuildTree{
		rootDir:     filepath.Join(configFileFolder, buildConfig.RootDir),
		rootNodes:   []*buildNode{},
		allNodes:    make(map[string]*buildNode),
		credentials: make(map[string]*credentialConfig),
	}
	for _, buildNodeConfig := range buildConfig.Build {
		node := &buildNode{
			buildRoot:  utils.ResolveDir(buildTree.rootDir, buildNodeConfig.From),
			name:       utils.FormatDockerName(buildNodeConfig.Name),
			tag:        buildNodeConfig.Tag,
			depend:     utils.FormatDockerName(buildNodeConfig.Depend),
			preBuild:   buildNodeConfig.PreBuild,
			postBuild:  buildNodeConfig.PostBuild,
			children:   []*buildNode{},
			dirty:      false,
			forceBuild: buildNodeConfig.ForceBuild,
		}
		buildTree.allNodes[node.name] = node
	}
	for _, credential := range buildConfig.Credentials {
		buildTree.credentials[credential.Name] = credential
	}
	return buildTree, nil
}

func readBuildConfig(fileContent []byte, variableMap map[string]string) (*config, error) {
	fileContentWithEnvExpanded := os.ExpandEnv(string(fileContent))
	tmpl, err := template.New("doriath").Parse(fileContentWithEnvExpanded)
	if err != nil {
		return nil, err
	}
	tmpl = tmpl.Option("missingkey=error")
	b := &bytes.Buffer{}
	err = tmpl.Execute(b, variableMap)
	if err != nil {
		return nil, err
	}
	buildConfig := &config{}
	err = yaml.Unmarshal(b.Bytes(), buildConfig)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot decode build file")
	}
	return buildConfig, nil
}

// Prepare checks the build tree for error and produces build steps
func (t *BuildTree) Prepare() error {
	for _, node := range t.allNodes {
		if node.depend == "" {
			t.rootNodes = append(t.rootNodes, node)
		} else {
			parent, ok := t.allNodes[node.depend]
			if !ok {
				return stacktrace.NewError("Dependency for %q not found: %q", node.name, node.depend)
			}
			parent.children = append(parent.children, node)
			err := t.cyclicCheck(node)
			if err != nil {
				return err
			}
		}
	}

	for _, node := range t.allNodes {
		err := t.assertDockerfile(node)
		if err != nil {
			return err
		}
	}
	for _, node := range t.rootNodes {
		err := t.dirtyCheck(node, false, false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Build builds all new images
func (t *BuildTree) Build() error {
	err := utils.DetectRequirement()
	if err != nil {
		return err
	}
	fmt.Println("Building new images")
	for _, node := range t.rootNodes {
		err = t.buildNodeAndChildren(node)
		if err != nil {
			return err
		}
	}
	return nil
}

// TryBuild tries to build new images locally, then delete them
func (t *BuildTree) TryBuild() error {
	err := utils.DetectRequirement()
	if err != nil {
		return err
	}
	fmt.Println("Try building new images")
	for _, node := range t.rootNodes {
		err = t.tryBuildNodeAndChildren(node)
		if err != nil {
			return err
		}
	}
	return nil
}

// Push pushes new images to registry
func (t *BuildTree) Push() error {
	err := t.Build()
	if err != nil {
		return err
	}
	fmt.Println("Logging in to registry")
	for _, credential := range t.credentials {
		err = utils.DockerLogin(credential.Registry, credential.Username, credential.Password)
		if err != nil {
			return err
		}
	}
	fmt.Println("Pushing new images")
	for _, node := range t.rootNodes {
		err = t.pushNodeAndChildren(node)
		if err != nil {
			return err
		}
	}
	return nil
}

// PrintTree prints the build tree
func (t *BuildTree) PrintTree(noColor bool) {
	for _, node := range t.rootNodes {
		t.printTree(node, 0, noColor)
	}
}

func (t *BuildTree) cyclicCheck(node *buildNode) error {
	nodes := make(utils.StringSet)
	current := node
	for {
		if nodes.Exists(current.name) {
			return stacktrace.Propagate(ErrCyclicDependency{current.name}, "Cyclic dependency found for %q", current.name)
		}
		nodes.Add(current.name)
		if current.depend == "" {
			return nil
		}
		parent, ok := t.allNodes[current.depend]
		if !ok {
			return stacktrace.Propagate(ErrDependencyMissing{node.name, node.depend}, "Dependency for %q not found: %q", node.name, node.depend)
		}
		current = parent
	}
}

func (t *BuildTree) assertDockerfile(node *buildNode) error {
	if node.depend == "" {
		return nil
	}
	// Check FROM:xxx and node dep
	imageInfo, err := utils.ExtractParentImageFromDockerfile(filepath.Join(node.buildRoot, "Dockerfile"))
	if err != nil {
		return err
	}
	if !utils.CompareDockerName(node.depend, imageInfo.FullName) {
		return stacktrace.Propagate(ErrMismatchDependencyImage{node.name, node.depend, imageInfo.FullName}, "Mismatch dependency for %q: %q in config but got %q in dockerfile", node.name, node.depend, imageInfo.FullName)
	}
	parentTag := t.allNodes[node.depend].tag
	if parentTag != imageInfo.Tag {
		return stacktrace.Propagate(ErrMismatchDependencyTag{node.name, node.depend, parentTag, imageInfo.Tag}, "Mismatch dependency image tag for %q (parent is %q): %q in config but got %q in dockerfile", node.name, node.depend, parentTag, imageInfo.Tag)
	}
	return nil
}

func (t *BuildTree) dirtyCheck(node *buildNode, parentIsDirty, parentIsForced bool) error {
	if parentIsForced || node.forceBuild == true {
		node.forceBuild = true
		node.dirty = true
	} else {
		imageInfo, err := utils.ExtractDockerImageInfo(node.name)
		if err != nil {
			return err
		}
		credential := t.credentials[imageInfo.RegistryName]
		if credential == nil {
			return stacktrace.Propagate(ErrMissingCredential{imageInfo.RegistryName}, "Cannot find credential for %s", imageInfo.RegistryName)
		}
		tagExists, err := utils.DockerCheckTagExists(imageInfo.ShortName, node.tag, &utils.DockerCredential{
			Registry: credential.Registry,
			Username: credential.Username,
			Password: credential.Password,
		})
		if err != nil {
			return err
		}
		if t.isProvided(node) {
			if !tagExists {
				return stacktrace.Propagate(ErrMissingTag{node.tag, node.name}, "Cannot find tag %q for provided image %q", node.tag, node.name)
			}
			node.dirty = false
		} else if parentIsDirty {
			node.dirty = true
			if tagExists {
				return stacktrace.Propagate(ErrImageTagOutdated{node.name}, "Image needs to be updated but still using old tag: %q", node.name)
			}
		} else {
			node.dirty = !tagExists
		}
	}

	for _, child := range node.children {
		err := t.dirtyCheck(child, node.dirty, node.forceBuild)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BuildTree) isProvided(node *buildNode) bool {
	return node.buildRoot == "provided"
}

func (t *BuildTree) needBuild(node *buildNode) bool {
	return node.dirty || node.forceBuild
}

func (t *BuildTree) buildNodeAndChildren(node *buildNode) error {
	if !t.needBuild(node) {
		fmt.Printf("====> Skipping %s\n", node.name)
	} else {
		fmt.Printf("====> Building %s:%s\n", node.name, node.tag)
		err := t.buildNode(node, node.tag)
		if err != nil {
			return err
		}
	}
	for _, child := range node.children {
		err := t.buildNodeAndChildren(child)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BuildTree) tryBuildNodeAndChildren(node *buildNode) error {
	if !t.needBuild(node) {
		fmt.Printf("====> Skipping %s\n", node.name)
	} else {
		randomTag := fmt.Sprintf("%s-%d", node.tag, time.Now().UnixNano())
		fmt.Printf("====> Building %s:%s\n", node.name, randomTag)
		err := t.buildNode(node, randomTag)
		if err != nil {
			return err
		}
		fmt.Printf("====> Removing %s:%s\n", node.name, randomTag)
		err = utils.DockerRMI(node.name, randomTag)
		if err != nil {
			utils.PrintError(err)
		}
	}
	for _, child := range node.children {
		err := t.tryBuildNodeAndChildren(child)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BuildTree) buildNode(node *buildNode, tag string) error {
	if node.preBuild != "" {
		err := utils.RunShellCommand(t.resolveShellCommandPath(t.rootDir, node.preBuild))
		if err != nil {
			return err
		}
	}
	err := utils.DockerBuild(node.name, tag, node.buildRoot)
	if node.postBuild != "" {
		utils.RunShellCommand(t.resolveShellCommandPath(t.rootDir, node.postBuild))
	}
	return err
}

func (t *BuildTree) resolveShellCommandPath(buildRoot, command string) string {
	if strings.HasPrefix(command, "/") {
		return command
	}
	if strings.HasPrefix(command, "./") || strings.HasPrefix(command, "../") {
		return buildRoot + "/" + command
	}
	return command
}

func (t *BuildTree) pushNodeAndChildren(node *buildNode) error {
	if !t.needBuild(node) {
		fmt.Printf("====> Skipping %s\n", node.name)
	} else {
		fmt.Printf("====> Pushing %s:%s\n", node.name, node.tag)
		err := utils.DockerPush(node.name, node.tag)
		if err != nil {
			return err
		}
	}
	for _, child := range node.children {
		err := t.pushNodeAndChildren(child)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BuildTree) printTree(node *buildNode, level int, noColor bool) {
	prefix := strings.Repeat("  ", level) + "-"
	var dirtyPrefix string
	var dirtyMark string
	var dirtySuffix string
	if t.needBuild(node) {
		dirtyMark = " (*)"
		if !noColor {
			dirtyPrefix = "\033[0;32m"
			dirtySuffix = "\033[0m"
		}
	}
	fmt.Printf("%s%s %s:%s%s%s\n", dirtyPrefix, prefix, node.name, node.tag, dirtyMark, dirtySuffix)
	for _, child := range node.children {
		t.printTree(child, level+1, noColor)
	}
}
