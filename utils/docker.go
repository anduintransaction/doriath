package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/palantir/stacktrace"
)

// Default values
const (
	DefaultRegistryName = "dockerhub"
	DefaultRegistry     = "https://registry.hub.docker.com"
)

var fromRegex = regexp.MustCompile("^[fF][rR][oO][mM]\\s+")

// DockerImageInfo stores common information for dockerfile
type DockerImageInfo struct {
	FullName     string
	ShortName    string
	RegistryName string
	Tag          string
}

// DockerCredential holds credential information for a docker registry request
type DockerCredential struct {
	Registry string
	Username string
	Password string
}

type dockerAuthInfo struct {
	authType string
	realm    string
	service  string
	scope    string
}

// FormatDockerName adds library/ if possible
func FormatDockerName(name string) string {
	if name == "" {
		return name
	}
	segments := strings.Split(name, "/")
	if len(segments) == 1 {
		return "library/" + name
	}
	return name
}

// CompareDockerName compares 2 docker image names
func CompareDockerName(a, b string) bool {
	return strings.TrimPrefix(a, "library/") == strings.TrimPrefix(b, "library/")
}

// ExtractDockerImageInfo extracts docker image info
func ExtractDockerImageInfo(fullnameWithTag string) (*DockerImageInfo, error) {
	imageInfo := &DockerImageInfo{}
	segments := strings.Split(fullnameWithTag, ":")
	if len(segments) > 2 {
		return nil, stacktrace.NewError("Invalid image full name: %q", fullnameWithTag)
	}
	imageInfo.FullName = segments[0]
	if len(segments) < 2 {
		imageInfo.Tag = "latest"
	} else {
		imageInfo.Tag = segments[1]
	}
	segments = strings.Split(imageInfo.FullName, "/")
	if len(segments) > 3 {
		return nil, stacktrace.NewError("Invalid image full name: %q", fullnameWithTag)
	} else if len(segments) < 3 {
		imageInfo.RegistryName = DefaultRegistryName
		imageInfo.ShortName = imageInfo.FullName
	} else {
		imageInfo.RegistryName = segments[0]
		imageInfo.ShortName = strings.Join(segments[1:], "/")
	}
	return imageInfo, nil
}

// ExtractParentImageFromDockerfile extracs image information from dockerfile
func ExtractParentImageFromDockerfile(filename string) (*DockerImageInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot open dockerfile %q", filename)
	}
	defer f.Close()
	var imageInfo *DockerImageInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if fromRegex.MatchString(line) {
			imageFullname := fromRegex.ReplaceAllString(line, "")
			imageInfo, err = ExtractDockerImageInfo(imageFullname)
			if err != nil {
				return nil, err
			}
		}
	}
	return imageInfo, nil
}

// DockerCheckTagExists checks if a tag exists on registry or not
func DockerCheckTagExists(shortName, tag string, credential *DockerCredential) (bool, error) {
	authInfo, err := dockerCheckTagFirstRequest(shortName, credential)
	if err != nil {
		return false, err
	}
	token, err := dockerRequestToken(shortName, authInfo, credential)
	if err != nil {
		return false, err
	}
	return dockerCheckTagSecondRequest(shortName, tag, authInfo.authType, token, credential)
}

// DockerImageExistsOnLocal .
func DockerImageExistsOnLocal(fullname string) (bool, error) {
	cmd := exec.Command("docker", "images", "-q", fullname)
	output, err := cmd.Output()
	if err != nil {
		return false, stacktrace.Propagate(err, "Cannot check image on local: %s", fullname)
	}
	if len(output) == 0 {
		return false, nil
	}
	return true, nil
}

// DockerLogin logins to docker registry
func DockerLogin(host, username, password string) error {
	var cmd *exec.Cmd
	if host == "" {
		cmd = exec.Command("docker", "login", "-u", username, "-p", password)
	} else {
		cmd = exec.Command("docker", "login", "-u", username, "-p", password, host)
	}
	errBuffer := &bytes.Buffer{}
	cmd.Stderr = errBuffer
	return stacktrace.Propagate(cmd.Run(), "Cannot login: %s", errBuffer.String())
}

// DockerBuild builds a docker image
func DockerBuild(name, tag, buildRoot string) error {
	cmd := exec.Command("docker", "build", "-t", name+":"+tag, buildRoot)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return stacktrace.Propagate(cmd.Run(), "Cannot build docker image")
}

// DockerPull .
func DockerPull(fullname string) error {
	existed, err := DockerImageExistsOnLocal(fullname)
	if err != nil {
		return err
	}
	if existed {
		return nil
	}
	cmd := exec.Command("docker", "pull", fullname)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return stacktrace.Propagate(cmd.Run(), "Cannot pull docker image")
}

// DockerPush pushes a docker image
func DockerPush(name, tag string) error {
	cmd := exec.Command("docker", "push", name+":"+tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return stacktrace.Propagate(cmd.Run(), "Cannot push docker image")
}

// DockerRMI removes a docker image
func DockerRMI(name, tag string) error {
	cmd := exec.Command("docker", "rmi", name+":"+tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return stacktrace.Propagate(cmd.Run(), "Cannot remove docker image")
}

// DockerTryRMI removes a docker image if exists, and will retry if necessary
func DockerTryRMI(name, tag string) error {
	return RetryWithFixedDelay(5*time.Second, 20, func() error {
		cmd := exec.Command("docker", "rmi", name+":"+tag)
		errOutput := &bytes.Buffer{}
		cmd.Stdout = os.Stdout
		cmd.Stderr = errOutput
		err := cmd.Run()
		if err == nil {
			return nil
		}
		if strings.Contains(errOutput.String(), "No such image") {
			return nil
		}
		return stacktrace.NewError("Cannot remove docker image: %s", errOutput.String())
	})
}

// DockerFindLatestTag .
func DockerFindLatestTag(imageInfo *DockerImageInfo, credential *DockerCredential) (string, error) {
	switch credential.Registry {
	case "https://gcr.io":
		return dockerFindGCRLatestTag(imageInfo, credential)
	default:
		return "", stacktrace.NewError("registry not supported: %q", credential.Registry)
	}
}

func dockerFindGCRLatestTag(imageInfo *DockerImageInfo, credential *DockerCredential) (string, error) {
	authInfo, err := dockerCheckTagFirstRequest(imageInfo.ShortName, credential)
	if err != nil {
		return "", err
	}
	token, err := dockerRequestToken(imageInfo.ShortName, authInfo, credential)
	if err != nil {
		return "", err
	}
	return dockerFindLatestTag(imageInfo, authInfo.authType, token, credential)
}

func getTagListURL(shortName string, credential *DockerCredential) string {
	var registry string
	if credential.Registry == "" {
		registry = DefaultRegistry
	} else {
		registry = credential.Registry
	}
	return registry + "/v2/" + shortName + "/tags/list"
}

func dockerCheckTagFirstRequest(shortName string, credential *DockerCredential) (*dockerAuthInfo, error) {
	return dockerFirstRequest(getTagListURL(shortName, credential), credential)
}

func dockerFirstRequest(url string, credential *DockerCredential) (*dockerAuthInfo, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot create request to %s", url)
	}
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot make http request to %s", url)
	}
	if response.StatusCode != http.StatusUnauthorized {
		return nil, stacktrace.NewError("Unexpected status code for request to %s: got %d", url, response.StatusCode)
	}
	authHeaderContent := response.Header.Get("Www-Authenticate")
	if authHeaderContent == "" {
		return nil, stacktrace.NewError("Empty Www-Authenticate header")
	}
	segments := strings.Split(authHeaderContent, " ")
	if len(segments) != 2 {
		return nil, stacktrace.NewError("Invalid Www-Authenticate header: %q", authHeaderContent)
	}
	authInfo := &dockerAuthInfo{
		authType: segments[0],
	}
	segments = strings.Split(segments[1], ",")
	for _, segment := range segments {
		subSegments := strings.Split(segment, "=")
		if len(subSegments) != 2 {
			return nil, stacktrace.NewError("Invalid Www-Authenticate header: %q, invalid segment %q", authHeaderContent, segment)
		}
		switch subSegments[0] {
		case "realm":
			authInfo.realm = strings.Trim(subSegments[1], "\"")
		case "service":
			authInfo.service = strings.Trim(subSegments[1], "\"")
		case "scope":
			authInfo.scope = strings.Trim(subSegments[1], "\"")
		}
	}
	return authInfo, nil
}

func dockerRequestToken(shortName string, authInfo *dockerAuthInfo, credential *DockerCredential) (string, error) {
	tokenQueryParams := url.Values{}
	tokenQueryParams.Add("service", authInfo.service)
	if authInfo.scope == "" {
		tokenQueryParams.Add("scope", "repository:"+shortName+":*")
	} else {
		tokenQueryParams.Add("scope", authInfo.scope)
	}
	tokenURL := authInfo.realm + "?" + tokenQueryParams.Encode()
	request, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot create token request %s", tokenURL)
	}
	request.SetBasicAuth(credential.Username, credential.Password)
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot make token request %s", tokenURL)
	}
	responseContent, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot read response content to %s", tokenURL)
	}
	if response.StatusCode != http.StatusOK {
		return "", stacktrace.NewError("Unexpected status code %d, response body is %s", response.StatusCode, string(responseContent))
	}
	var tokenJSON struct {
		Token string `json:"token"`
	}
	err = json.Unmarshal(responseContent, &tokenJSON)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot parse body content: %s", string(responseContent))
	}
	return tokenJSON.Token, nil
}

func dockerCheckTagSecondRequest(shortName, tag, authType, token string, credential *DockerCredential) (bool, error) {
	tagListURL := getTagListURL(shortName, credential)
	request, err := http.NewRequest("GET", tagListURL, nil)
	if err != nil {
		return false, stacktrace.Propagate(err, "Cannot create request to %s", tagListURL)
	}
	request.Header.Add("Authorization", authType+" "+token)
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return false, stacktrace.Propagate(err, "Cannot make request to %s", tagListURL)
	}
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, stacktrace.Propagate(err, "Cannot read body of request to %s", tagListURL)
	}
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, stacktrace.NewError("Unexpected status: %d, response body: %s", response.StatusCode, string(responseBody))
	}
	var tagResponse struct {
		Tags []string `json:"tags"`
	}
	err = json.Unmarshal(responseBody, &tagResponse)
	if err != nil {
		return false, stacktrace.Propagate(err, "Cannot decode response body: %s", string(responseBody))
	}
	found := false
	for _, remoteTag := range tagResponse.Tags {
		if tag == remoteTag {
			found = true
			break
		}
	}
	return found, nil
}

type gcrTagList struct {
	Manifest map[string]*gcrManifest `json:"manifest"`
}

type gcrManifest struct {
	Tag []string `json:"tag"`
}

func dockerFindLatestTag(imageInfo *DockerImageInfo, authType, token string, credential *DockerCredential) (string, error) {
	tagListURL := getTagListURL(imageInfo.ShortName, credential)
	request, err := http.NewRequest("GET", tagListURL, nil)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot create request to %s", tagListURL)
	}
	request.Header.Add("Authorization", authType+" "+token)
	request.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot make request to %s", tagListURL)
	}
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", stacktrace.Propagate(err, "Cannot read body of request to %s", tagListURL)
	}
	if response.StatusCode != http.StatusOK {
		return "", stacktrace.NewError("Unexpected status: %d, response body: %s", response.StatusCode, string(responseBody))
	}
	tagList := &gcrTagList{}
	err = json.Unmarshal(responseBody, tagList)
	if err != nil {
		return "", stacktrace.Propagate(err, "cannot decode json %s", string(responseBody))
	}
	for _, tagInfo := range tagList.Manifest {
		foundLatestTag := false
		for _, tag := range tagInfo.Tag {
			if tag == "latest" {
				foundLatestTag = true
				break
			}
		}
		if foundLatestTag {
			for _, tag := range tagInfo.Tag {
				if tag != "latest" {
					return tag, nil
				}
			}
		}
	}
	return "", stacktrace.NewError("cannot find latest tag")
}
