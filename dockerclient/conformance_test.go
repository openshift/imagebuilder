// +build conformance

package dockerclient

import (
	"archive/tar"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/openshift/imagebuilder/dockerfile/command"
	"github.com/openshift/imagebuilder/dockerfile/parser"

	"github.com/openshift/imagebuilder"
)

var compareLayers = flag.Bool("compare-layers", false, "If true, compare each generated layer for equivalence")

type conformanceTest struct {
	Name       string
	Dockerfile string
	Git        string
	Mounts     []Mount
	ContextDir string
	Output     []*regexp.Regexp
	Args       map[string]string
	Ignore     []ignoreFunc
	PostClone  func(dir string) error
}

func TestMount(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "dockerbuild-conformance-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	e := NewClientExecutor(c)
	defer func() {
		for _, err := range e.Release() {
			t.Errorf("%v", err)
		}
	}()

	out := &bytes.Buffer{}
	e.Out, e.ErrOut = out, out
	e.Tag = filepath.Base(tmpDir)
	e.TransientMounts = []Mount{
		{SourcePath: "testdata/volume/", DestinationPath: "/tmp/test"},
	}
	b := imagebuilder.NewBuilder(nil)
	node, err := imagebuilder.ParseFile("testdata/Dockerfile.mount")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Prepare(b, node, ""); err != nil {
		t.Fatal(err)
	}
	if err := e.Execute(b, node); err != nil {
		t.Fatal(err)
	}

	expected := `91 /tmp/test/Dockerfile 644 regular file 0 0
4 /tmp/test/file 644 regular file 0 0
5 /tmp/test/file2 644 regular file 0 0
`

	if out.String() != expected {
		t.Errorf("Unexpected build output:\n%s", out.String())
	}
}

func TestCopyFrom(t *testing.T) {
	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name   string
		create string
		copy   string
		extra  string
		expect string
	}{
		{name: "copy file to root", create: "touch /a /b", copy: "/a /", expect: "[[ -f /a ]]"},
		{name: "copy file to same file", create: "touch /a", copy: "/a /a", expect: "[[ -f /a ]]"},
		{name: "copy file to workdir", create: "touch /a", extra: "WORKDIR /b", copy: "/a .", expect: "[[ -f /b/a ]]"},
		{name: "copy file to workdir rename", create: "touch /a", extra: "WORKDIR /b", copy: "/a ./b", expect: "[[ -f /b/b ]]"},
		{name: "copy folder contents to higher level", create: "mkdir -p /a/b && touch /a/b/1 /a/b/2", copy: "/a/b/ /b/", expect: "[[ -f /b/1 && -f /b/2 && ! -e /a ]]"},
		{name: "copy wildcard folder contents to higher level", create: "mkdir -p /a/b && touch /a/b/1 /a/b/2", copy: "/a/b/* /b/", expect: "ls -al /b/1 /b/2 /b && ! ls -al /a /b/a /b/b"},
		{name: "copy folder with dot contents to higher level", create: "mkdir -p /a/b && touch /a/b/1 /a/b/2", copy: "/a/b/. /b/", expect: "ls -al /b/1 /b/2 /b && ! ls -al /a /b/a /b/b"},
		{name: "copy root file to different root name", create: "touch /b", copy: "/b /a", expect: "ls -al /a && ! ls -al /b"},
		{name: "copy nested file to different root name", create: "mkdir -p /a && touch /a/b", copy: "/a/b /a", expect: "ls -al /a && ! ls -al /b"},
		{name: "copy hard links to excluded file", create: "mkdir -p /a/b/c && touch /a/b/c/d && ln /a/b/c/d /a/b/d && ln /a/b/c/d /a/b/e", extra: "RUN mkdir -p /f/g", copy: "/a/b/d /a/b/e /f/g/", expect: "ls -al /f && ls -al /f/g && ls -al /f/g/d /f/g/e"},
		{name: "copy file to deeper directory with explicit slash", create: "mkdir -p /a && touch /a/1", copy: "/a/1 /a/b/c/", expect: "ls -al /a/b/c/1 && ! ls -al /a/b/1"},
		{name: "copy file to deeper directory without explicit slash", create: "mkdir -p /a && touch /a/1", copy: "/a/1 /a/b/c", expect: "ls -al /a/b/c && ! ls -al /a/b/1"},
		{name: "copy directory to deeper directory without explicit slash", create: "mkdir -p /a && touch /a/1", copy: "/a /a/b/c", expect: "ls -al /a/b/c/1 && ! ls -al /a/b/1"},
		{name: "copy item from directory that is a symbolic link", create: "mkdir -p /a && touch /a/1 && ln -s /a /b", copy: "b/1 /a/b/c", expect: "ls -al /a/b/c && ! ls -al /a/b/1"},
		{name: "copy item from directory that is a symbolic link", create: "mkdir -p /a && touch /a/1 && ln -s a /c", copy: "/c/1 /a/b/c", expect: "ls -al /a/b/c && ! ls -al /a/b/1"},
		{name: "copy directory to root without explicit slash", create: "mkdir -p /a && touch /a/1", copy: "a /a", expect: "ls -al /a/1 && ! ls -al /a/a"},
		{name: "copy directory trailing to root without explicit slash", create: "mkdir -p /a && touch /a/1", copy: "a/. /a", expect: "ls -al /a/1 && ! ls -al /a/a"},
	}
	for i, testCase := range testCases {
		name := fmt.Sprintf("%d", i)
		if len(testCase.name) > 0 {
			name = testCase.name
		}
		test := testCase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := NewClientExecutor(c)
			defer func() {
				for _, err := range e.Release() {
					t.Errorf("%v", err)
				}
			}()

			out := &bytes.Buffer{}
			e.Out, e.ErrOut = out, out
			b := imagebuilder.NewBuilder(nil)
			dockerfile := fmt.Sprintf(`
				FROM busybox AS base
				RUN %s
				FROM busybox
				%s
				COPY --from=base %s
				RUN %s
			`, test.create, test.extra, test.copy, test.expect,
			)
			t.Log(dockerfile)
			node, err := imagebuilder.ParseDockerfile(strings.NewReader(dockerfile))
			if err != nil {
				t.Fatal(err)
			}

			stages, err := imagebuilder.NewStages(node, b)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.Stages(b, stages, ""); err != nil {
				t.Log(out.String())
				t.Fatal(err)
			}
		})
	}
}

func TestShell(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "dockerbuild-conformance-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	e := NewClientExecutor(c)
	defer func() {
		for _, err := range e.Release() {
			t.Errorf("%v", err)
		}
	}()

	out := &bytes.Buffer{}
	e.Out, e.ErrOut = out, out
	e.Directory = tmpDir
	e.Tag = filepath.Base(tmpDir)
	b := imagebuilder.NewBuilder(nil)
	node, err := imagebuilder.ParseFile("testdata/Dockerfile.shell")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Prepare(b, node, ""); err != nil {
		t.Fatal(err)
	}
	if err := e.Execute(b, node); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "+ env\n") {
		t.Errorf("Unexpected build output:\n%s", out.String())
	}
}

func TestMultiStageBase(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "dockerbuild-conformance-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	e := NewClientExecutor(c)
	defer func() {
		for _, err := range e.Release() {
			t.Errorf("%v", err)
		}
	}()

	out := &bytes.Buffer{}
	e.Out, e.ErrOut = out, out
	e.Directory = tmpDir
	e.Tag = filepath.Base(tmpDir)
	node, err := imagebuilder.ParseFile("testdata/Dockerfile.reusebase")
	if err != nil {
		t.Fatal(err)
	}

	b := imagebuilder.NewBuilder(nil)
	stages, err := imagebuilder.NewStages(node, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Stages(b, stages, ""); err != nil {
		t.Fatal(err)
	}
	if out.String() != "/1\n" {
		t.Errorf("Unexpected build output:\n%s", out.String())
	}
}

// TestConformance* compares the result of running the direct build against a
// sequential docker build. A dockerfile and git repo is loaded, then each step
// in the file is run sequentially, committing after each step. The generated
// image.Config and the resulting filesystems are compared. The next step reuses
// the previously generated layer and performs an incremental diff. This ensures
// that each step is functionally equivalent.
//
// Deviations:
// * Builds run at different times
//   * Modification timestamps are ignored on files
//   * Some processes (gem install) result in files created in the image that
//     have different content because of that (timestamps in files). We treat
//     a file that is identical except for size within 10 bytes and neither old
//     or new is zero bytes to be identical.
// * Docker container commit with ENV FOO=BAR and a Docker build with line
//   ENV FOO=BAR will generate an image with FOO=BAR in different positions
//   (commit places the variable first, build: last). We try to align the
//   generated environment variable to ensure they are equal.
// * The parent image ID is ignored.
//
// TODO: .dockerignore
// TODO: check context dir
// TODO: ONBUILD
// TODO: ensure that the final built image has the right UIDs
//
func TestConformanceInternal(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	testCases := []conformanceTest{
		{
			Name:       "directory",
			ContextDir: "testdata/dir",
		},
		{
			Name:       "copy to dir",
			ContextDir: "testdata/copy",
		},
		{
			Name:       "copy chown",
			ContextDir: "testdata/copychown",
		},
		{
			Name:       "copy empty 1",
			ContextDir: "testdata/copyempty",
			Dockerfile: "Dockerfile",
		},
		{
			Name:       "copy empty 2",
			ContextDir: "testdata/copyempty",
			Dockerfile: "Dockerfile2",
		},
		{
			Name:       "copy dir",
			ContextDir: "testdata/copydir",
		},
		{
			Name:       "copy to renamed file",
			ContextDir: "testdata/copyrename",
		},
		{
			Name:       "copy subdirectory 1",
			ContextDir: "testdata/copyblahblub",
			Dockerfile: "Dockerfile",
		},
		{
			Name:       "copy subdirectory 2",
			ContextDir: "testdata/copyblahblub",
			Dockerfile: "Dockerfile2",
		},
		{
			Name:       "copy subdirectory 3",
			ContextDir: "testdata/copyblahblub",
			Dockerfile: "Dockerfile3",
		},
		{
			Name:       "directory with slash",
			ContextDir: "testdata/overlapdir",
			Dockerfile: "Dockerfile.with_slash",
		},
		{
			Name:       "directory without slash",
			ContextDir: "testdata/overlapdir",
			Dockerfile: "Dockerfile.without_slash",
		},
		// TODO: Fix this test
		// {
		// 	ContextDir: "testdata/ignore",
		// },
		{
			Name:       "environment",
			Dockerfile: "testdata/Dockerfile.env",
		},
		{
			Name:       "edgecases",
			Dockerfile: "testdata/Dockerfile.edgecases",
		},
		{
			Name:       "exposed_default",
			Dockerfile: "testdata/Dockerfile.exposedefault",
		},
		{
			Name:       "add",
			Dockerfile: "testdata/Dockerfile.add",
		},
		{
			Name:       "add archives",
			ContextDir: "testdata/add",
		},
		{
			Name:       "copy archives",
			ContextDir: "testdata/add",
			Dockerfile: "Dockerfile.copy",
		},
		{
			Name:       "add directories with archives",
			ContextDir: "testdata/add",
			Dockerfile: "Dockerfile.addall",
		},
		{
			Name:       "add directories with archives 2",
			ContextDir: "testdata/add",
			Dockerfile: "Dockerfile.addslash",
		},
		{
			Name:       "run with JSON",
			Dockerfile: "testdata/Dockerfile.run.args",
			Output: []*regexp.Regexp{
				// docker outputs colorized output
				regexp.MustCompile(`(?m)(\[0m|^)inner outer$`),
				regexp.MustCompile(`(?m)(\[0m|^)first second$`),
				regexp.MustCompile(`(?m)(\[0m|^)third fourth$`),
				regexp.MustCompile(`(?m)(\[0m|^)fifth sixth$`),
			},
		},
		{
			Name:       "shell",
			Dockerfile: "testdata/Dockerfile.shell",
		},
		{
			Name:       "args",
			Dockerfile: "testdata/Dockerfile.args",
			Args:       map[string]string{"BAR": "first"},
		},
		/*{ // uncomment when docker allows this
			Dockerfile: "testdata/Dockerfile.args",
			Args:       map[string]string{"BAZ": "first"},
		},*/
		{
			Name:       "wildcard",
			ContextDir: "testdata/wildcard",
		},
		{
			Name:       "wildcard leading path",
			ContextDir: "./testdata/wildcard",
		},
		{
			Name:       "wildcard absolute path",
			ContextDir: filepath.Join(pwd, "testdata", "wildcard"),
		},
		{
			Name:       "volume",
			ContextDir: "testdata/volume",
		},
		{
			Name:       "volumerun",
			ContextDir: "testdata/volumerun",
		},
		{
			Name:       "novolume",
			Dockerfile: "testdata/Dockerfile.novolume",
		},
		{
			Name:       "novolumenorun",
			Dockerfile: "testdata/Dockerfile.novolumenorun",
		},
		{
			Name:       "noworkdir",
			Dockerfile: "testdata/Dockerfile.noworkdir",
		},
		{
			Name:       "volumeexists",
			Dockerfile: "testdata/Dockerfile.volumeexists",
		},
		{
			Name:       "multistage 1",
			ContextDir: "testdata",
			Dockerfile: "Dockerfile.multistage",
		},
		{
			Name:       "multistage reuse base",
			ContextDir: "testdata",
			Dockerfile: "Dockerfile.reusebase",
		},
		{
			Name:       "multistage 2",
			ContextDir: "testdata/multistage",
			Dockerfile: "Dockerfile",
		},
		{
			Name:       "multistage copy",
			ContextDir: "testdata/copyfrom",
		},
		{
			Name:       "multistageconfiginheritance",
			ContextDir: "testdata/multistage",
			Dockerfile: "Dockerfile.env",
		},
	}

	for i, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			c, err := docker.NewClientFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			conformanceTester(t, c, test, i, *compareLayers)
		})
	}
}

// TestConformanceExternal applies external repo testing that may be more expensive or
// change more frequently.
func TestConformanceExternal(t *testing.T) {
	testCases := []conformanceTest{
		{
			Name: "ownership change under COPY",
			// Tests user ownership change under COPY
			Git: "https://github.com/openshift/ruby-hello-world.git",
		},
		{
			Name: "dockerfile custom location",
			// Tests Non-default location dockerfile
			Dockerfile: "Dockerfile.build",
			Git:        "https://github.com/docker-library/hello-world.git",
			PostClone: func(dir string) error {
				return os.Remove(filepath.Join(dir, ".dockerignore"))
			},
		},
		{
			Name: "copy and env interaction",
			// Tests COPY and other complex interactions of ENV
			ContextDir: "14/alpine",
			Dockerfile: "Dockerfile",
			Git:        "https://github.com/docker-library/postgres.git",
			Ignore: []ignoreFunc{
				func(a, b *tar.Header) bool {
					switch {
					case (a != nil) == (b != nil):
						return false
					case a != nil:
						return strings.HasPrefix(a.Name, "etc/ssl/certs/")
					case b != nil:
						return strings.HasPrefix(b.Name, "etc/ssl/certs/")
					default:
						return false
					}
				},
			},
		},
	}

	for i, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			c, err := docker.NewClientFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			conformanceTester(t, c, test, i, *compareLayers)
		})
	}
}

func TestTransientMount(t *testing.T) {
	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	e := NewClientExecutor(c)
	defer func() {
		for _, err := range e.Release() {
			t.Errorf("%v", err)
		}
	}()

	e.AllowPull = true
	e.Directory = "testdata"
	e.TransientMounts = []Mount{
		{SourcePath: "testdata/dir", DestinationPath: "/mountdir"},
		{SourcePath: "testdata/Dockerfile.env", DestinationPath: "/mountfile"},
	}
	e.Tag = fmt.Sprintf("conformance%d", rand.Int63())

	defer e.removeImage(e.Tag)

	out := &bytes.Buffer{}
	e.Out = out
	b := imagebuilder.NewBuilder(nil)
	node, err := imagebuilder.ParseDockerfile(bytes.NewBufferString("FROM busybox\nRUN ls /mountdir/subdir\nRUN cat /mountfile\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Build(b, node, ""); err != nil {
		t.Fatalf("unable to build image: %v", err)
	}
	if !strings.Contains(out.String(), "ENV name=value\n") {
		t.Errorf("did not find expected output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "file2\n") {
		t.Errorf("did not find expected output:\n%s", out.String())
	}

	result, err := testContainerOutput(c, e.Tag, []string{"/bin/sh", "-c", "ls -al /mountdir"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "subdir") {
		t.Errorf("did not find expected output:\n%s", result)
	}
	result, err = testContainerOutput(c, e.Tag, []string{"/bin/sh", "-c", "cat /mountfile"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "ENV name=value\n") {
		t.Errorf("did not find expected output:\n%s", result)
	}
}

func testContainerOutput(c *docker.Client, tag string, command []string) (string, error) {
	container, err := c.CreateContainer(docker.CreateContainerOptions{
		Name: tag + "-test",
		Config: &docker.Config{
			Image:      tag,
			Entrypoint: command,
			Cmd:        nil,
		},
	})
	if err != nil {
		return "", err
	}
	defer c.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID})
	if err := c.StartContainer(container.ID, nil); err != nil {
		return "", err
	}
	code, err := c.WaitContainer(container.ID)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("unrecognized exit code: %d", code)
	}
	out := &bytes.Buffer{}
	if err := c.Logs(docker.LogsOptions{Container: container.ID, Stdout: true, OutputStream: out}); err != nil {
		return "", fmt.Errorf("unable to get logs: %v", err)
	}
	return out.String(), nil
}

func conformanceTester(t *testing.T, c *docker.Client, test conformanceTest, i int, deep bool) {
	dockerfile := test.Dockerfile
	if len(dockerfile) == 0 {
		dockerfile = "Dockerfile"
	}
	tmpDir, err := ioutil.TempDir("", "dockerbuild-conformance-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir := tmpDir
	contextDir := filepath.Join(dir, test.ContextDir)
	dockerfilePath := filepath.Join(dir, test.ContextDir, dockerfile)

	// clone repo or copy the Dockerfile
	var input string
	switch {
	case len(test.Git) > 0:
		input = test.Git
		cmd := exec.Command("git", "clone", test.Git, dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("unable to clone %q: %v\n%s", test.Git, err, out)
			return
		}

		if test.PostClone != nil {
			if err := test.PostClone(dir); err != nil {
				t.Errorf("unable to fixup clone: %v", err)
				return
			}
		}
		dir = contextDir

	case len(test.ContextDir) > 0:
		hardlinks := new(hardlinkChecker)
		if err := filepath.Walk(filepath.Join("", test.ContextDir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			dest := filepath.Join(dir, path)
			if info.IsDir() {
				if err := os.MkdirAll(dest, info.Mode()); err != nil {
					return err
				}
				return os.Chtimes(dest, info.ModTime(), info.ModTime())
			}
			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				linkTarget, err := os.Readlink(path)
				if err != nil {
					return err
				}
				return os.Symlink(linkTarget, dest)
			}
			if info.Mode().IsRegular() {
				if hardlinkTarget, ok := hardlinks.Check(info, dest); ok {
					return os.Link(hardlinkTarget, dest)
				}
				if _, err := fileutils.CopyFile(path, dest); err != nil {
					return err
				}
				if err := os.Chmod(dest, info.Mode()&os.ModePerm); err != nil {
					return err
				}
				return os.Chtimes(dest, info.ModTime(), info.ModTime())
			}
			return fmt.Errorf("%s: %w", dest, syscall.ENOTSUP)
		}); err != nil {
			t.Fatal(err)
		}

		contextDir = filepath.Join(dir, test.ContextDir)
		dockerfilePath = filepath.Join(contextDir, "Dockerfile")
		if len(test.Dockerfile) > 0 {
			dockerfilePath = filepath.Join(contextDir, test.Dockerfile)
		}
		dir = contextDir
		input = dockerfilePath

	default:
		dockerfilePath = filepath.Join(dir, "Dockerfile")
		input = dockerfilePath
		if _, err := fileutils.CopyFile(filepath.Join("", dockerfile), dockerfilePath); err != nil {
			t.Fatal(err)
		}
		dockerfile = "Dockerfile"
		dir = contextDir
	}

	// read the dockerfile
	data, err := ioutil.ReadFile(dockerfilePath)
	if err != nil {
		t.Errorf("%d: unable to read Dockerfile %q: %v", i, input, err)
		return
	}
	node, err := imagebuilder.ParseDockerfile(bytes.NewBuffer(data))
	if err != nil {
		t.Errorf("%d: can't parse Dockerfile %q: %v", i, input, err)
		return
	}
	builder := imagebuilder.NewBuilder(nil)
	stages, err := imagebuilder.NewStages(node, builder)
	if err != nil {
		t.Errorf("%d: error parsing Dockerfile %q: %v", i, input, err)
		return
	}
	nameFormat := "conformance-dockerbuild-%d-%s-%d-%d"

	var toDelete []string

	ignoreSmallFileChange := func(a, b *tar.Header) bool {
		if a == nil || b == nil {
			return false
		}
		diff := a.Size - b.Size
		if differOnlyByFileSize(a, b, 10) {
			t.Logf("WARNING: %s differs only in size by %d bytes, probably a timestamp value change", a.Name, diff)
			return true
		}
		return false
	}

	dockerOut := &bytes.Buffer{}
	imageOut := &bytes.Buffer{}
	exclude, _ := imagebuilder.ParseDockerignore(contextDir)

	if deep {
		// dockerfileWithFrom returns the contents of a new docker file with a different
		// FROM as the first line, and any --from= arguments in COPY or ADD instructions
		// replaced with the names of images that we expect to have created at the end
		// of the stages that built them
		dockerfileWithFrom := func(from string, steps []*parser.Node, currentStageIndex int) (string, error) {
			lines := []string{}
			lines = append(lines, fmt.Sprintf("FROM %s", from))
			for _, step := range steps {
				switch strings.ToUpper(step.Value) {
				case strings.ToUpper(command.Add), strings.ToUpper(command.Copy):
					line := strings.ToUpper(step.Value)
					for _, flag := range step.Flags {
						// replace --from=stageName|stageNumber with --from=stageFinalImage
						if strings.HasPrefix(flag, "--from=") {
							stageLabel := strings.TrimPrefix(flag, "--from=")
							if b, ok := stages.ByName(stageLabel); ok {
								otherStage := fmt.Sprintf(nameFormat, i, "docker", b.Position, len(b.Node.Children))
								flag = "--from=" + otherStage
							} else if stageIndex, err := strconv.Atoi(stageLabel); err == nil {
								if stageIndex >= currentStageIndex {
									return "", fmt.Errorf("%q references not-yet-built stage", step.Original)
								}
								b := stages[stageIndex]
								otherStage := fmt.Sprintf(nameFormat, i, "docker", b.Position, len(b.Node.Children))
								flag = "--from=" + otherStage
							}
						}
						line = line + " " + flag
					}
					next := step.Next
					for next != nil {
						line = line + " " + next.Value
						next = next.Next
					}
					lines = append(lines, line)
				default:
					lines = append(lines, step.Original)
				}
			}
			return strings.Join(lines, "\n"), nil
		}

		// execute each stage on both Docker build and the direct
		// builder, comparing as we go
		for j := range stages {
			// execute thru each step in this stage on both Docker
			// build and the direct builder, comparing as we go
			stageBase, err := builder.From(stages[j].Node)
			if err != nil {
				t.Fatalf("%d: %v", j, err)
			}
			// if the base is the result of a previous stage,
			// resolve it to that stage's final image here
			if b, ok := stages.ByName(stageBase); ok {
				stageBase = fmt.Sprintf(nameFormat, i, "docker", b.Position, len(b.Node.Children))
			}
			steps := stages[j].Node.Children
			for k := range steps {
				// construct the Dockerfile
				testFile, err := dockerfileWithFrom(stageBase, steps[0:k+1], j)
				if err != nil {
					t.Fatalf("%d: unable to reconstruct Dockerfile %q: %v", i, dockerfilePath, err)
				}

				nameDirect := fmt.Sprintf(nameFormat, i, "direct", j, k+1)
				nameDocker := fmt.Sprintf(nameFormat, i, "docker", j, k+1)

				// run docker build for this stage thru this step
				if err := ioutil.WriteFile(dockerfilePath, []byte(testFile), 0600); err != nil {
					t.Fatalf("%d: unable to update Dockerfile %q: %v", i, dockerfilePath, err)
				}
				in, err := archive.TarWithOptions(dir, &archive.TarOptions{IncludeFiles: []string{"."}, ExcludePatterns: exclude})
				if err != nil {
					t.Fatalf("%d: unable to generate build context %q: %v", i, dockerfilePath, err)
				}
				var args []docker.BuildArg
				for k, v := range test.Args {
					args = append(args, docker.BuildArg{Name: k, Value: v})
				}
				if err := c.BuildImage(docker.BuildImageOptions{
					Name:                nameDocker,
					Dockerfile:          dockerfile,
					RmTmpContainer:      true,
					ForceRmTmpContainer: true,
					InputStream:         in,
					OutputStream:        dockerOut,
					BuildArgs:           args,
					NoCache:             len(test.Output) > 0,
				}); err != nil {
					in.Close()
					data, _ := ioutil.ReadFile(dockerfilePath)
					t.Fatalf("%d: unable to build Docker image %q: %v\n%s\n%s", i, test.Git, err, string(data), dockerOut)
				}
				in.Close()
				toDelete = append([]string{nameDocker}, toDelete...)

				// run direct build of this stage thru this step
				e := NewClientExecutor(c)
				defer func() {
					for _, err := range e.Release() {
						t.Errorf("%v", err)
					}
				}()
				e.Out, e.ErrOut = imageOut, imageOut
				e.Directory = dir
				e.Tag = nameDirect
				b := imagebuilder.NewBuilder(test.Args)
				node, err := imagebuilder.ParseDockerfile(bytes.NewBufferString(testFile))
				if err != nil {
					t.Fatalf("%d: %v", i, err)
				}
				if err := e.Build(b, node, ""); err != nil {
					t.Fatalf("%d: failed to build through step %d/%d in dockerfile %q: %s\n%s", i, j, k, dockerfilePath, steps[k].Original, imageOut)
				}
				toDelete = append([]string{nameDirect}, toDelete...)

				// only compare filesystem on layers that change the filesystem
				mutation := steps[k].Value == command.Add || steps[k].Value == command.Copy || steps[k].Value == command.Run
				// metadata must be strictly equal
				if !equivalentImages(
					t, c, nameDocker, nameDirect, mutation,
					metadataEqual,
					append(ignoreFuncs{ignoreSmallFileChange}, test.Ignore...)...,
				) {
					data, _ := ioutil.ReadFile(dockerfilePath)
					t.Logf("Dockerfile:\n%s", data)
					t.Fatalf("%d: layered Docker build was not equivalent to direct layer image metadata %s", i, input)
				}
			}
		}
	} else {
		// run docker build
		in, err := archive.TarWithOptions(dir, &archive.TarOptions{IncludeFiles: []string{"."}, ExcludePatterns: exclude})
		if err != nil {
			t.Errorf("%d: unable to generate build context %q: %v", i, dockerfilePath, err)
			return
		}
		stageSteps := stages[len(stages)-1].Node.Children
		nameDocker := fmt.Sprintf(nameFormat, i, "docker", len(stages)-1, len(stageSteps))
		var args []docker.BuildArg
		for k, v := range test.Args {
			args = append(args, docker.BuildArg{Name: k, Value: v})
		}
		if err := c.BuildImage(docker.BuildImageOptions{
			Name:                nameDocker,
			Dockerfile:          dockerfile,
			RmTmpContainer:      true,
			ForceRmTmpContainer: true,
			InputStream:         in,
			OutputStream:        dockerOut,
			BuildArgs:           args,
			NoCache:             len(test.Output) > 0,
		}); err != nil {
			in.Close()
			t.Errorf("%d: unable to build Docker image %q: %v\n%s", i, test.Git, err, dockerOut)
			return
		}
		in.Close()
		toDelete = append([]string{nameDocker}, toDelete...)

		// run direct build
		b := imagebuilder.NewBuilder(test.Args)
		node, err := imagebuilder.ParseDockerfile(bytes.NewBuffer(data))
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		stages, err := imagebuilder.NewStages(node, b)
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		if len(stages) == 0 {
			t.Error("parsing Dockerfile produced no stages")
			return
		}
		nameDirect := fmt.Sprintf(nameFormat, i, "direct", len(stages)-1, len(stageSteps))
		e := NewClientExecutor(c)
		defer func() {
			for _, err := range e.Release() {
				t.Errorf("%v", err)
			}
		}()
		e.Out, e.ErrOut = imageOut, imageOut
		e.Directory = dir
		e.Tag = nameDirect
		lastExecutor, err := e.Stages(b, stages, "")
		if err != nil {
			t.Errorf("%v", err)
			return
		}
		if err := lastExecutor.Commit(stages[len(stages)-1].Builder); err != nil {
			t.Errorf("%d: failed to build complete image in %q: %v\n%s", i, input, err, imageOut)
		} else {
			toDelete = append([]string{nameDirect}, toDelete...)
			if !equivalentImages(
				t, c, nameDocker, nameDirect, true,
				// metadata should be loosely equivalent, but because we squash and because of limitations
				// in docker commit, there are some differences
				metadataLayerEquivalent,
				append(ignoreFuncs{
					ignoreSmallFileChange,
					// the direct dockerfile contains all steps, the layered image is synthetic from our previous
					// test and so only contains the last layer
					ignoreDockerfileSize(dockerfile),
				}, test.Ignore...)...,
			) {
				t.Errorf("%d: full Docker build was not equivalent to squashed image metadata %s", i, input)
			}
		}
	}

	badOutput := false
	for _, re := range test.Output {
		if !re.MatchString(dockerOut.String()) {
			t.Errorf("Docker did not output %v", re)
			badOutput = true
		}
		if !re.MatchString(imageOut.String()) {
			t.Errorf("Imagebuilder did not output %v", re)
			badOutput = true
		}
	}
	if badOutput {
		t.Logf("Output mismatch:\nDocker:\n---\n%s\n---\nImagebuilder:\n---\n%s\n---", hex.Dump(dockerOut.Bytes()), hex.Dump(imageOut.Bytes()))
	}

	for _, s := range toDelete {
		c.RemoveImageExtended(s, docker.RemoveImageOptions{Force: true})
	}
}

// ignoreFunc returns true if the difference between the two can be ignored
type ignoreFunc func(a, b *tar.Header) bool

type ignoreFuncs []ignoreFunc

func (fns ignoreFuncs) Ignore(a, b *tar.Header) bool {
	for _, fn := range fns {
		if fn(a, b) {
			return true
		}
	}
	return false
}

// metadataFunc returns true if the metadata is equivalent
type metadataFunc func(a, b *docker.Config) bool

func normalizeOutputMetadata(a, b *docker.Config) {
	// old docker servers can report no args escaped
	if !a.ArgsEscaped && b.ArgsEscaped {
		b.ArgsEscaped = false
	}
	if a.Entrypoint == nil && len(b.Entrypoint) == 0 {
		// we are forced to set Entrypoint [] to reset the entrypoint
		b.Entrypoint = nil
	}
	if len(a.Labels) == 0 && len(b.Labels) == 0 {
		a.Labels = nil
		b.Labels = nil
	}
	// Serialization of OnBuild is omitempty, which means it may be nil or empty depending on
	// docker version
	if len(a.OnBuild) == len(b.OnBuild) && len(a.OnBuild) == 0 {
		b.OnBuild = a.OnBuild
	}
}

// metadataEqual checks that the metadata of two images is directly equivalent.
func metadataEqual(a, b *docker.Config) bool {
	// compare output metadata
	a.Image, b.Image = "", ""
	a.Hostname, b.Hostname = "", ""
	e1, e2 := envMap(a.Env), envMap(b.Env)
	if !reflect.DeepEqual(e1, e2) {
		return false
	}
	normalizeOutputMetadata(a, b)
	a.Env, b.Env = nil, nil
	if !reflect.DeepEqual(a, b) {
		return false
	}
	return true
}

// metadataLayerEquivalent returns true if the last layer of a is equivalent to b, assuming
// that b is squashed over multiple layers, and a is not. b, for instance, will have an empty
// slice entrypoint, while a would have a nil entrypoint.
func metadataLayerEquivalent(a, b *docker.Config) bool {
	normalizeOutputMetadata(a, b)
	if len(a.OnBuild) == 1 && len(b.OnBuild) > 0 && a.OnBuild[0] == b.OnBuild[len(b.OnBuild)-1] {
		// a layered file will only contain the last OnBuild statement
		b.OnBuild = a.OnBuild
	}
	return metadataEqual(a, b)
}

// equivalentImages executes the provided checks against two docker images, returning true
// if the images are equivalent, and recording a test suite error in any other condition.
func equivalentImages(t *testing.T, c *docker.Client, a, b string, testFilesystem bool, metadataFn metadataFunc, ignoreFns ...ignoreFunc) bool {
	imageA, err := c.InspectImage(a)
	if err != nil {
		t.Errorf("can't get image %q: %v", a, err)
		return false
	}
	imageB, err := c.InspectImage(b)
	if err != nil {
		t.Errorf("can't get image %q: %v", b, err)
		return false
	}

	if !metadataFn(imageA.Config, imageB.Config) {
		t.Errorf("generated image metadata did not match (%s, %s):\n%#v\n%#v", a, b, imageA.Config, imageB.Config)
		return false
	}

	// for mutation commands, check the layer diff
	if testFilesystem {
		differs, onlyA, onlyB, err := compareImageFS(c, a, b)
		if err != nil {
			t.Errorf("can't calculate FS differences %q: %v", a, err)
			return false
		}
		for k, v := range differs {
			if ignoreFuncs(ignoreFns).Ignore(v[0].Header, v[1].Header) {
				delete(differs, k)
				continue
			}
			t.Errorf("%s and %s differ at %s:\n%#v\n%#v", a, b, k, v[0].Header, v[1].Header)
		}
		for k, v := range onlyA {
			if ignoreFuncs(ignoreFns).Ignore(v.Header, nil) {
				delete(onlyA, k)
				continue
			}
		}
		for k, v := range onlyB {
			if ignoreFuncs(ignoreFns).Ignore(nil, v.Header) {
				delete(onlyB, k)
				continue
			}
		}
		if len(onlyA)+len(onlyB)+len(differs) > 0 {
			t.Errorf("a(%s)=%v b(%s)=%v diff=%v", a, onlyA, b, onlyB, differs)
			return false
		}
	}
	return true
}

// envMap returns a map from a list of environment variables.
func envMap(env []string) map[string]string {
	out := make(map[string]string)
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			out[envVar] = ""
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}

// differOnlyByFileSize returns true iff the headers differ only by size, but
// that differences is less than within bytes.
func differOnlyByFileSize(a, b *tar.Header, within int64) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Size == b.Size {
		return false
	}

	diff := a.Size - b.Size
	if diff < 0 {
		diff = diff * -1
	}
	if diff < within && a.Size != 0 && b.Size != 0 {
		a.Size = b.Size
		if reflect.DeepEqual(a, b) {
			return true
		}
	}
	return false
}

// ignore Dockerfile being different, artifact of this test
func ignoreDockerfileSize(dockerfile string) ignoreFunc {
	return func(a, b *tar.Header) bool {
		if a == nil || b == nil {
			return false
		}
		if !strings.HasSuffix(a.Name, dockerfile) {
			return false
		}
		if a.Size != b.Size {
			a.Size = b.Size
			return reflect.DeepEqual(a, b)
		}
		return false
	}
}

// compareImageFS exports the file systems of two images and returns a map
// of files that differ in any way (modification time excluded), only exist in
// image A, or only existing in image B.
func compareImageFS(c *docker.Client, a, b string) (differ map[string][]tarHeader, onlyA, onlyB map[string]tarHeader, err error) {
	fsA, err := imageFSMetadata(c, a)
	if err != nil {
		return nil, nil, nil, err
	}
	fsB, err := imageFSMetadata(c, b)
	if err != nil {
		return nil, nil, nil, err
	}
	differ = make(map[string][]tarHeader)
	onlyA = make(map[string]tarHeader)
	onlyB = fsB
	for k, v1 := range fsA {
		v2, ok := fsB[k]
		if !ok {
			onlyA[k] = v1
			continue
		}
		delete(onlyB, k)
		// we ignore modification time differences
		v1.ModTime = time.Time{}
		v2.ModTime = time.Time{}
		if !reflect.DeepEqual(v1, v2) {
			differ[k] = []tarHeader{v1, v2}
		}
	}
	return differ, onlyA, onlyB, nil
}

type tarHeader struct {
	*tar.Header
}

func (h tarHeader) String() string {
	th := h.Header
	if th == nil {
		return "nil"
	}
	return fmt.Sprintf("<%d %s>", th.Size, th.FileInfo().Mode())
}

// imageFSMetadata creates a container and reads the filesystem metadata out of the archive.
func imageFSMetadata(c *docker.Client, name string) (map[string]tarHeader, error) {
	container, err := c.CreateContainer(docker.CreateContainerOptions{Name: name + "-export", Config: &docker.Config{Image: name}})
	if err != nil {
		return nil, err
	}
	defer c.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, RemoveVolumes: true, Force: true})

	ch := make(chan struct{})
	result := make(map[string]tarHeader)
	r, w := io.Pipe()
	go func() {
		defer close(ch)
		out := tar.NewReader(r)
		for {
			h, err := out.Next()
			if err != nil {
				if err == io.EOF {
					w.Close()
				} else {
					w.CloseWithError(err)
				}
				break
			}
			result[h.Name] = tarHeader{h}
		}
	}()
	if err := c.ExportContainer(docker.ExportContainerOptions{ID: container.ID, OutputStream: w}); err != nil {
		return nil, err
	}
	<-ch
	return result, nil
}

type hardlinkChecker struct {
	known map[hardlinkCheckerKey]string
}

func (h *hardlinkChecker) Check(info os.FileInfo, name string) (string, bool) {
	if h.known == nil {
		h.known = make(map[hardlinkCheckerKey]string)
	}
	key := h.makeHardlinkCheckerKey(info)
	if key != nil {
		if name, ok := h.known[*key]; ok {
			return name, ok
		}
		h.known[*key] = name
	}
	return "", false
}
