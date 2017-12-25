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
build:
  - name: "human/aragorn"
    from: "./human/aragorn"
    tag: "1.2.0"
    depent: "elf/elrond"
  - name: "elf/arwen"
    from: "./elf/arwen"
    tag: "3.1.4"
    depent: "elf/elrond"
  - name: "wizard/gandalf"
    from: "./wizard/gandalf"
    tag: "0.5.2"
  - name: "elf/elrond"
    from: "./elf/elrond"
    tag: "2.1.0"
credentials:
  - name: dockerhub
    username: "$YOUR_USERNAME"
    password: "${YOUR_PASSWORD}"
  - name: gcr.io
    registry: "https://gcr.io/v2/"
    username: "_json_key"
    password: "**********************"
```
