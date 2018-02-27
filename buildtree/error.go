package buildtree

import "fmt"

type ErrCyclicDependency struct {
	Name string
}

func (e ErrCyclicDependency) Error() string {
	return fmt.Sprintf("cyclic dependency found for %q", e.Name)
}

type ErrDependencyMissing struct {
	Name   string
	Depend string
}

func (e ErrDependencyMissing) Error() string {
	return fmt.Sprintf("dependency for %q not found: %q", e.Name, e.Depend)
}

type ErrMismatchDependencyImage struct {
	Name           string
	Depend         string
	ActualFullname string
}

func (e ErrMismatchDependencyImage) Error() string {
	return fmt.Sprintf("mismatch dependency for %q: %q in config but got %q in dockerfile", e.Name, e.Depend, e.ActualFullname)
}

type ErrMismatchDependencyTag struct {
	Name        string
	Depend      string
	ExpectedTag string
	ActualTag   string
}

func (e ErrMismatchDependencyTag) Error() string {
	return fmt.Sprintf("mismatch dependency image tag for %q (parent is %q): %q in config but got %q in dockerfile", e.Name, e.Depend, e.ExpectedTag, e.ActualTag)
}

type ErrMissingCredential struct {
	RegistryName string
}

func (e ErrMissingCredential) Error() string {
	return fmt.Sprintf("missing credential for %q", e.RegistryName)
}

type ErrMissingTag struct {
	Tag  string
	Name string
}

func (e ErrMissingTag) Error() string {
	return fmt.Sprintf("cannot find tag %q for provided image %q", e.Tag, e.Name)
}

type ErrImageTagOutdated struct {
	Name string
}

func (e ErrImageTagOutdated) Error() string {
	return fmt.Sprintf("image needs to be updated but still using old tag: %q", e.Name)
}
