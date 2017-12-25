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

	"github.com/palantir/stacktrace"
)

const (
	DefaultRegistryName = "dockerhub"
	DefaultRegistry     = "https://registry-1.docker.io/v2"
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
	token, err := dockerRequestToken(authInfo, credential)
	if err != nil {
		return false, err
	}
	return dockerCheckTagSecondRequest(shortName, tag, authInfo.authType, token, credential)
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

// DockerPush pushes a docker image
func DockerPush(name, tag string) error {
	cmd := exec.Command("docker", "push", name+":"+tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return stacktrace.Propagate(cmd.Run(), "Cannot push docker image")
}

func getTagListURL(shortName string, credential *DockerCredential) string {
	var registry string
	if credential.Registry == "" {
		registry = DefaultRegistry
	} else {
		registry = credential.Registry
	}
	return registry + "/" + shortName + "/tags/list"
}

func dockerCheckTagFirstRequest(shortName string, credential *DockerCredential) (*dockerAuthInfo, error) {
	tagListURL := getTagListURL(shortName, credential)
	request, err := http.NewRequest("GET", tagListURL, nil)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot create tag list request to %s", tagListURL)
	}
	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Cannot make http request to %s", tagListURL)
	}
	if response.StatusCode != http.StatusUnauthorized {
		return nil, stacktrace.NewError("Unexpected status code for request to %s: got %d", tagListURL, response.StatusCode)
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

func dockerRequestToken(authInfo *dockerAuthInfo, credential *DockerCredential) (string, error) {
	tokenQueryParams := url.Values{}
	tokenQueryParams.Add("service", authInfo.service)
	tokenQueryParams.Add("scope", authInfo.scope)
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
