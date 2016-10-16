package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	dockertypes "github.com/docker/engine-api/types"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/imagebuilder/dockerclient"
)

func main() {
	log.SetFlags(0)
	options := dockerclient.NewClientExecutor(nil)
	var dockerfilePath string
	var mountSpecs stringSliceFlag

	flag.StringVar(&dockerfilePath, "dockerfile", dockerfilePath, "An optional path to a Dockerfile to use.")
	flag.Var(&mountSpecs, "mount", "An optional list of files and directories to mount during the build. Use SRC:DST syntax for each path.")
	flag.BoolVar(&options.AllowPull, "allow-pull", true, "Pull the images that are not present.")
	flag.BoolVar(&options.IgnoreUnrecognizedInstructions, "ignore-unrecognized-instructions", true, "If an unrecognized Docker instruction is encountered, warn but do not fail the build.")

	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		log.Fatalf("You must provide two arguments: DIRECTORY and IMAGE_NAME")
	}

	options.Directory = args[0]
	options.Tag = args[1]
	if len(dockerfilePath) == 0 {
		dockerfilePath = filepath.Join(options.Directory, "Dockerfile")
	}

	var mounts []dockerclient.Mount
	for _, s := range mountSpecs {
		segments := strings.Split(s, ":")
		if len(segments) != 2 {
			log.Fatalf("--mount must be of the form SOURCE:DEST")
		}
		mounts = append(mounts, dockerclient.Mount{SourcePath: segments[0], DestinationPath: segments[1]})
	}
	options.TransientMounts = mounts

	options.Out, options.ErrOut = os.Stdout, os.Stderr
	options.AuthFn = func(name string) ([]dockertypes.AuthConfig, bool) {
		return nil, false
	}
	options.LogFn = func(format string, args ...interface{}) {
		if glog.V(2) {
			log.Printf("Builder: "+format, args...)
		} else {
			fmt.Fprintf(options.ErrOut, "--> %s\n", fmt.Sprintf(format, args...))
		}
	}

	// Accept ARGS on the command line
	arguments := make(map[string]string)

	if err := build(dockerfilePath, options, arguments); err != nil {
		log.Fatal(err.Error())
	}
}

func build(dockerfilePath string, e *dockerclient.ClientExecutor, arguments map[string]string) error {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatalf("No connection to Docker available: %v", err)
	}
	e.Client = client

	f, err := os.Open(dockerfilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// TODO: handle signals
	if err := e.Cleanup(e.Container); err != nil {
		fmt.Fprintf(e.ErrOut, "error: Unable to clean up build: %v\n", err)
	}

	return e.Build(f, arguments)
}

type stringSliceFlag []string

func (f *stringSliceFlag) Set(s string) error {
	*f = append(*f, s)
	return nil
}

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, " ")
}
