package imagebuilder

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/containerd/containerd/platforms"
	docker "github.com/fsouza/go-dockerclient"
	buildkitparser "github.com/moby/buildkit/frontend/dockerfile/parser"
)

func TestDispatchArgDefaultBuiltins(t *testing.T) {
	mybuilder := *NewBuilder(make(map[string]string))
	args := []string{"TARGETPLATFORM"}
	if err := arg(&mybuilder, args, nil, nil, "", nil); err != nil {
		t.Errorf("arg error: %v", err)
	}
	args = []string{"BUILDARCH"}
	if err := arg(&mybuilder, args, nil, nil, "", nil); err != nil {
		t.Errorf("arg(2) error: %v", err)
	}
	localspec := platforms.DefaultSpec()
	expectedArgs := []string{
		"BUILDARCH=" + localspec.Architecture,
		"TARGETPLATFORM=" + localspec.OS + "/" + localspec.Architecture,
	}
	got := mybuilder.Arguments()
	sort.Strings(got)
	if !reflect.DeepEqual(got, expectedArgs) {
		t.Errorf("Expected %v, got %v\n", expectedArgs, got)
	}
}

func TestDispatchArgTargetPlatform(t *testing.T) {
	mybuilder := *NewBuilder(make(map[string]string))
	args := []string{"TARGETPLATFORM=linux/arm/v7"}
	if err := arg(&mybuilder, args, nil, nil, "", nil); err != nil {
		t.Errorf("arg error: %v", err)
	}
	expectedArgs := []string{
		"TARGETARCH=arm",
		"TARGETOS=linux",
		"TARGETPLATFORM=linux/arm/v7",
		"TARGETVARIANT=v7",
	}
	got := mybuilder.Arguments()
	sort.Strings(got)
	if !reflect.DeepEqual(got, expectedArgs) {
		t.Errorf("Expected %v, got %v\n", expectedArgs, got)
	}
}

func TestDispatchArgTargetPlatformBad(t *testing.T) {
	mybuilder := *NewBuilder(make(map[string]string))
	args := []string{"TARGETPLATFORM=bozo"}
	err := arg(&mybuilder, args, nil, nil, "", nil)
	expectedErr := errors.New("error parsing TARGETPLATFORM argument")
	if !reflect.DeepEqual(err, expectedErr) {
		t.Errorf("Expected %v, got %v\n", expectedErr, err)
	}
}

func TestDispatchCopy(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--from=builder"}
	original := "COPY --from=builder /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "builder",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "",
			Chmod:    "",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, got %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyHeredoc(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}
	args := []string{"<<robots.txt <<humans.txt", "/test/"}
	flagArgs := []string{}
	original := "COPY <<robots.txt <<humans.txt /test/"
	heredocs := []buildkitparser.Heredoc{{Name: "robots.txt", FileDescriptor: 0, Chomp: false, Content: "(robots content)"}, {Name: "humans.txt", FileDescriptor: 0, Chomp: false, Content: "(humans content)"}}
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, heredocs); err != nil {
		t.Errorf("copy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"<<robots.txt <<humans.txt"},
			Dest:     "/test/", // destination must contain a trailing slash
			Download: false,
			Chown:    "",
			Chmod:    "",
			Files:    []File{{Name: "robots.txt", Data: "(robots content)"}, {Name: "humans.txt", Data: "(humans content)"}},
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, got %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchAddHeredoc(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	args := []string{"<<EOF", "/index.html"}
	flagArgs := []string{}
	original := "ADD <<EOF /index.html"
	heredocs := []buildkitparser.Heredoc{{Name: "EOF", Expand: true, Content: "(your index page goes here)"}}
	if err := add(&mybuilder, args, nil, flagArgs, original, heredocs); err != nil {
		t.Errorf("add error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"<<EOF"},
			Dest:     "/index.html",
			Download: true,
			Files:    []File{{Name: "EOF", Data: "(your index page goes here)"}},
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchRunHeredoc(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{}
	args := []string{"<< EOF"}
	original := "RUN <<EOF"
	heredocs := []buildkitparser.Heredoc{{Name: "EOF", Content: "echo \"Hello\" >> /hello\necho \"World\" >> /hello"}}

	if err := run(&mybuilder, args, nil, flags, original, heredocs); err != nil {
		t.Errorf("run error: %v", err)
	}
	expectedPendingRuns := []Run{
		{
			Shell: true,
			Args:  args,
			Files: []File{{Name: "EOF", Data: "echo \"Hello\" >> /hello\necho \"World\" >> /hello"}},
		},
	}

	if !reflect.DeepEqual(mybuilder.PendingRuns, expectedPendingRuns) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingRuns, mybuilder.PendingRuns)
	}

}

func TestDispatchRunHeredocWithCommand(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{}
	args := []string{"python3 << EOF"}
	original := "RUN python3 <<EOF"
	heredocs := []buildkitparser.Heredoc{{Name: "EOF", Content: "with open(\"/hello\", \"w\") as f:\n    print(\"Hello\", file=f)\n    print(\"Something\", file=f)"}}

	if err := run(&mybuilder, args, nil, flags, original, heredocs); err != nil {
		t.Errorf("run error: %v", err)
	}
	expectedPendingRuns := []Run{
		{
			Shell: true,
			Args:  args,
			Files: []File{{Name: "EOF", Data: "with open(\"/hello\", \"w\") as f:\n    print(\"Hello\", file=f)\n    print(\"Something\", file=f)"}},
		},
	}

	if !reflect.DeepEqual(mybuilder.PendingRuns, expectedPendingRuns) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingRuns, mybuilder.PendingRuns)
	}
}

func TestDispatchCopyChown(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	mybuilder2 := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	// Test Bad chown values
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=1376:1376"}
	original := "COPY --chown=1376:1376 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to not match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}

	// Test Good chown values
	flagArgs = []string{"--chown=6731:6731"}
	original = "COPY --chown=6731:6731 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder2, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}
	expectedPendingCopies = []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder2.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder2.PendingCopies)
	}
}

func TestDispatchCopyChmod(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	mybuilder2 := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	// Test Bad chmod values
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=888"}
	original := "COPY --chmod=888 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil)
	chmod := "888"
	convErr := checkChmodConversion(chmod)
	if err != nil && convErr != nil && err.Error() != convErr.Error() {
		t.Errorf("Expected chmod conversion error, instead got error: %v", err)
	}
	if err == nil || convErr == nil {
		t.Errorf("Expected conversion error for chmod %s", chmod)
	}

	// Test Good chmod values
	flagArgs = []string{"--chmod=777"}
	original = "COPY --chmod=777 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder2, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "",
			Chmod:    "777",
		},
	}
	if !reflect.DeepEqual(mybuilder2.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder2.PendingCopies)
	}
}

func TestDispatchAddChownWithEnvironment(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Env: []string{"CHOWN_VAL=6731:6731"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=${CHOWN_VAL}"}
	original := "ADD --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := add(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchAddChmodWithEnvironment(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Env: []string{"CHMOD_VAL=755"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=${CHMOD_VAL}"}
	original := "ADD --chmod=${CHMOD_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := add(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Chmod:    "755",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchAddChownWithArg(t *testing.T) {
	argsMap := make(map[string]string)
	allowedArgs := make(map[string]bool)
	argsMap["CHOWN_VAL"] = "6731:6731"
	allowedArgs["CHOWN_VAL"] = true
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Args:        argsMap,
		AllowedArgs: allowedArgs,
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=${CHOWN_VAL}"}
	original := "ADD --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := add(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchAddChmodWithArg(t *testing.T) {
	argsMap := make(map[string]string)
	allowedArgs := make(map[string]bool)
	argsMap["CHMOD_VAL"] = "644"
	allowedArgs["CHMOD_VAL"] = true
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Args:        argsMap,
		AllowedArgs: allowedArgs,
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=${CHMOD_VAL}"}
	original := "ADD --chmod=${CHMOD_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := add(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Chmod:    "644",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChownWithEnvironment(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Env: []string{"CHOWN_VAL=6731:6731"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=${CHOWN_VAL}"}
	original := "COPY --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChmodWithEnvironment(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Env: []string{"CHMOD_VAL=660"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=${CHMOD_VAL}"}
	original := "COPY --chmod=${CHMOD_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chmod:    "660",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChownWithArg(t *testing.T) {
	argsMap := make(map[string]string)
	argsMap["CHOWN_VAL"] = "6731:6731"
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Args: argsMap,
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=${CHOWN_VAL}"}
	original := "COPY --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChmodWithArg(t *testing.T) {
	argsMap := make(map[string]string)
	argsMap["CHMOD_VAL"] = "444"
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Args: argsMap,
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=${CHMOD_VAL}"}
	original := "COPY --chmod=${CHMOD_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chmod:    "444",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChownWithSameArgAndEnv(t *testing.T) {
	argsMap := make(map[string]string)
	argsMap["CHOWN_VAL"] = "4321:4321"
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Args: argsMap,
		Env:  []string{"CHOWN_VAL=6731:6731"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=${CHOWN_VAL}"}
	original := "COPY --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchCopyChmodWithSameArgAndEnv(t *testing.T) {
	argsMap := make(map[string]string)
	argsMap["CHMOD_VAL"] = "777"
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
		Args: argsMap,
		Env:  []string{"CHMOD_VAL=444"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=${CHMOD_VAL}"}
	original := "COPY --chmod=${CHMOD_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("copy error: %v", err)
	}

	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chmod:    "444",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchAddChown(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	mybuilder2 := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	// Test Bad chown values
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=1376:1376"}
	original := "ADD --chown=1376:1376 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"
	if err := add(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "6731:6731",
		},
	}
	if reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to not match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}

	// Test Good chown values
	flagArgs = []string{"--chown=6731:6731"}
	original = "ADD --chown=6731:6731 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"
	if err := add(&mybuilder2, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}
	expectedPendingCopies = []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Chown:    "6731:6731",
		},
	}
	if !reflect.DeepEqual(mybuilder2.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder2.PendingCopies)
	}
}

func TestDispatchAddChmod(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	mybuilder2 := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	// Test Bad chmod values
	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chmod=rwxrwxrwx"}
	original := "ADD --chmod=rwxrwxrwx /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"
	err := add(&mybuilder, args, nil, flagArgs, original, nil)
	chmod := "rwxrwxrwx"
	convErr := checkChmodConversion(chmod)
	if err != nil && convErr != nil && err.Error() != convErr.Error() {
		t.Errorf("Expected chmod conversion error, instead got error: %v", err)
	}
	if err == nil || convErr == nil {
		t.Errorf("Expected conversion error for chmod %s", chmod)
	}

	// Test Good chmod values
	flagArgs = []string{"--chmod=755"}
	original = "ADD --chmod=755 /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"
	if err := add(&mybuilder2, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Chmod:    "755",
		},
	}
	if !reflect.DeepEqual(mybuilder2.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder2.PendingCopies)
	}
}

func TestDispatchAddChecksum(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "alpine",
		},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--checksum=checksum"}
	original := "ADD --checksum=checksum /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"
	if err := add(&mybuilder, args, nil, flagArgs, original, nil); err != nil {
		t.Errorf("add error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: true,
			Checksum: "checksum",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingCopies, mybuilder.PendingCopies)
	}
}

func TestDispatchRunFlags(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{"--mount=type=bind,target=/foo"}
	args := []string{"echo \"stuff\""}
	original := "RUN --mount=type=bind,target=/foo echo \"stuff\""

	if err := run(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("run error: %v", err)
	}
	expectedPendingRuns := []Run{
		{
			Shell:  true,
			Args:   args,
			Mounts: []string{"type=bind,target=/foo"},
		},
	}

	if !reflect.DeepEqual(mybuilder.PendingRuns, expectedPendingRuns) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingRuns, mybuilder.PendingRuns)
	}

}

func TestDispatchNetworkFlags(t *testing.T) {
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{"--network=none"}
	args := []string{"echo \"stuff\""}
	original := "RUN --network=none echo \"stuff\""

	if err := run(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("run error: %v", err)
	}
	expectedPendingRuns := []Run{
		{
			Shell:   true,
			Args:    args,
			Network: "none",
		},
	}

	if !reflect.DeepEqual(mybuilder.PendingRuns, expectedPendingRuns) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingRuns, mybuilder.PendingRuns)
	}
}

func TestDispatchRunFlagsWithArgs(t *testing.T) {
	argsMap := make(map[string]string)
	allowedArgs := make(map[string]bool)
	argsMap["TYPE"] = "bind"
	allowedArgs["TYPE"] = true
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
		Args:        argsMap,
		AllowedArgs: allowedArgs,
	}

	flags := []string{"--mount=type=${TYPE},target=/foo"}
	args := []string{"echo \"stuff\""}
	original := "RUN --mount=type=${TYPE},target=/foo echo \"stuff\""

	if err := run(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("run error: %v", err)
	}
	expectedPendingRuns := []Run{
		{
			Shell:  true,
			Args:   args,
			Mounts: []string{"type=bind,target=/foo"},
		},
	}

	if !reflect.DeepEqual(mybuilder.PendingRuns, expectedPendingRuns) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingRuns, mybuilder.PendingRuns)
	}
	// Following run should not resolve correctly and type should be equal to "" i.e blank
	mybuilder = Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}
	if err := run(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("run error: %v", err)
	}
	expectedBadPendingRuns := []Run{
		{
			Shell:  true,
			Args:   args,
			Mounts: []string{"type=,target=/foo"},
		},
	}

	if !reflect.DeepEqual(mybuilder.PendingRuns, expectedBadPendingRuns) {
		t.Errorf("Expected %v, to match %v\n", expectedPendingRuns, mybuilder.PendingRuns)
	}
}

func TestDispatchFromFlags(t *testing.T) {
	expectedPlatform := "linux/arm64"
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{"--platform=linux/arm64"}
	args := []string{""}
	original := "FROM --platform=linux/arm64 busybox"

	if err := from(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("from error: %v", err)
	}

	if mybuilder.Platform != expectedPlatform {
		t.Errorf("Expected %v, to match %v\n", expectedPlatform, mybuilder.Platform)
	}
}

func TestDispatchFromFlagsAndUseBuiltInArgs(t *testing.T) {
	expectedPlatform := localspec.OS + "/" + localspec.Architecture
	if localspec.Variant != "" {
		expectedPlatform += "/" + localspec.Variant
	}
	mybuilder := Builder{
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{"--platform=$BUILDPLATFORM"}
	args := []string{""}
	original := "FROM --platform=$BUILDPLATFORM busybox"

	if err := from(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("from error: %v", err)
	}

	if mybuilder.Platform != expectedPlatform {
		t.Errorf("Expected %v, to match %v\n", expectedPlatform, mybuilder.Platform)
	}
}

func TestDispatchFromFlagsAndUseHeadingArgs(t *testing.T) {
	expectedPlatform := "foo/bar"
	mybuilder := Builder{
		HeadingArgs: map[string]string{
			"TMP": expectedPlatform,
		},
		RunConfig: docker.Config{
			WorkingDir: "/root",
			Cmd:        []string{"/bin/sh"},
			Image:      "busybox",
		},
	}

	flags := []string{"--platform=$TMP"}
	args := []string{""}
	original := "FROM --platform=$TMP busybox"

	if err := from(&mybuilder, args, nil, flags, original, nil); err != nil {
		t.Errorf("from error: %v", err)
	}

	if mybuilder.Platform != expectedPlatform {
		t.Errorf("Expected %v, to match %v\n", expectedPlatform, mybuilder.Platform)
	}
}
