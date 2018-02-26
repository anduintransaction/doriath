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
	rootNodes   []*buildNode
	allNodes    map[string]*buildNode
	credentials map[string]*credentialConfig
}

type buildNode struct {
	buildRoot string
	name      string
	tag       string
	depend    string
	children  []*buildNode
	dirty     bool
}

type config struct {
	RootDir     string              `yaml:"root_dir"`
	Build       []*buildNodeConfig  `yaml:"build"`
	Credentials []*credentialConfig `yaml:"credentials"`
}

type buildNodeConfig struct {
	Name   string `yaml:"name"`
	From   string `yaml:"from"`
	Tag    string `yaml:"tag"`
	Depend string `yaml:"depend"`
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
	return readBuildTree(fileContent, variableMap)
}

// ReadBuildTreeFromFile reads BuildTree from a build file
func ReadBuildTreeFromFile(buildFile string, variableMap map[string]string) (*BuildTree, error) {
	fileContent, err := ioutil.ReadFile(buildFile)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot read build file %q", buildFile)
	}
	return readBuildTree(fileContent, variableMap)
}

func readBuildTree(fileContent []byte, variableMap map[string]string) (*BuildTree, error) {
	buildConfig, err := readBuildConfig(fileContent, variableMap)
	if err != nil {
		return nil, err
	}
	buildTree := &BuildTree{
		rootNodes:   []*buildNode{},
		allNodes:    make(map[string]*buildNode),
		credentials: make(map[string]*credentialConfig),
	}
	for _, buildNodeConfig := range buildConfig.Build {
		node := &buildNode{
			buildRoot: utils.ResolveDir(buildConfig.RootDir, buildNodeConfig.From),
			name:      utils.FormatDockerName(buildNodeConfig.Name),
			tag:       buildNodeConfig.Tag,
			depend:    utils.FormatDockerName(buildNodeConfig.Depend),
			children:  []*buildNode{},
			dirty:     false,
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
		err := t.dirtyCheck(node, false)
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
			return stacktrace.NewError("Cyclic dependency found for %q", current.name)
		}
		nodes.Add(current.name)
		if current.depend == "" {
			return nil
		}
		parent, ok := t.allNodes[current.depend]
		if !ok {
			return stacktrace.NewError("Dependency for %q not found: %q", node.name, node.depend)
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
		return stacktrace.NewError("Mismatch dependency for %q: %q in config but got %q in dockerfile", node.name, node.depend, imageInfo.FullName)
	}
	parentTag := t.allNodes[node.depend].tag
	if parentTag != imageInfo.Tag {
		return stacktrace.NewError("Mismatch dependency image tag for %q (parent is %q): %q in config but got %q in dockerfile", node.name, node.depend, parentTag, imageInfo.Tag)
	}
	return nil
}

func (t *BuildTree) dirtyCheck(node *buildNode, parentIsDirty bool) error {
	imageInfo, err := utils.ExtractDockerImageInfo(node.name)
	if err != nil {
		return err
	}
	credential := t.credentials[imageInfo.RegistryName]
	if credential == nil {
		return stacktrace.NewError("Cannot find credential for %s", imageInfo.RegistryName)
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
			return stacktrace.NewError("Cannot find tag %q for provided image %q", node.tag, node.name)
		}
		node.dirty = false
	} else if parentIsDirty {
		node.dirty = true
		if tagExists {
			return stacktrace.NewError("Image needs to be updated but still using old tag: %s", node.name)
		}
	} else {
		node.dirty = !tagExists
	}

	for _, child := range node.children {
		err = t.dirtyCheck(child, node.dirty)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *BuildTree) isProvided(node *buildNode) bool {
	return node.buildRoot == "provided"
}

func (t *BuildTree) buildNodeAndChildren(node *buildNode) error {
	if !node.dirty {
		fmt.Printf("====> Skipping %s\n", node.name)
	} else {
		fmt.Printf("====> Building %s:%s\n", node.name, node.tag)
		err := utils.DockerBuild(node.name, node.tag, node.buildRoot)
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
	if !node.dirty {
		fmt.Printf("====> Skipping %s\n", node.name)
	} else {
		randomTag := fmt.Sprintf("%s-%d", node.tag, time.Now().UnixNano())
		fmt.Printf("====> Building %s:%s\n", node.name, randomTag)
		err := utils.DockerBuild(node.name, randomTag, node.buildRoot)
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

func (t *BuildTree) pushNodeAndChildren(node *buildNode) error {
	if !node.dirty {
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
	if node.dirty {
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
