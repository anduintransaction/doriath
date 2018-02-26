# Simple automated build graph for docker

# Installation

Check https://github.com/anduintransaction/doriath/releases

# How to run

 - Create a `doriath.yml` file in your project
 - `doriath dryrun` to check for build steps and possible errors
 - `doriath build` to build all docker images locally
 - `doriath push` to push all images

# Sample configuration file:

```yaml
root_dir: .
pull:
  - "ubuntu:16.04"
  - "centos:7"
build:
  - name: "ubuntu"
    tag: "16.04"
    from: "provided"
  - name: "human/aragorn"
    from: "./human/aragorn"
    tag: "1.2.0"
    depend: "elf/elrond"
  - name: "elf/arwen"
    from: "./elf/arwen"
    tag: "3.1.4"
    depend: "elf/elrond"
  - name: "wizard/gandalf"
    from: "./wizard/gandalf"
    tag: "0.5.2"
  - name: "elf/elrond"
    from: "./elf/elrond"
    tag: "2.1.0"
    depend: "ubuntu"
    prebuild: "./init-elrond.sh" // Run this script file before building image
    postbuild: "./finalize-elrond.sh" // Run this script file after building image
    forcebuild: true // Always build and push this image, skip checking for existance from registry
credentials:
  - name: dockerhub
    username: "$YOUR_USERNAME" // Use environment variable
    password: "${YOUR_PASSWORD}" // Here too
  - name: gcr.io
    registry: "https://gcr.io/v2/"
    username: "_json_key"
    password: "**********************"
```

# Using variable for configuration file

The config file supports go-template syntax. For example:

```yaml
root_dir: .
build:
  - name: "my-image"
    tag: "{{.myImageTag}}"
    from: "./my-image"
```

Then you can pass the value of `myImageTag` from command line:

`doriath build --variable myImageTag=2.1`
