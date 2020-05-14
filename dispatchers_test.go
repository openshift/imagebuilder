package imagebuilder

import (
	"errors"
	"github.com/containerd/containerd/platforms"
	"reflect"
	"sort"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

func TestDispatchArgDefaultBuiltins(t *testing.T) {
	mybuilder := *NewBuilder(make(map[string]string))
	args := []string{"TARGETPLATFORM"}
	if err := arg(&mybuilder, args, nil, nil, ""); err != nil {
		t.Errorf("arg error: %v", err)
	}
	args = []string{"BUILDARCH"}
	if err := arg(&mybuilder, args, nil, nil, ""); err != nil {
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
	if err := arg(&mybuilder, args, nil, nil, ""); err != nil {
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
	err := arg(&mybuilder, args, nil, nil, "")
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
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
	}
	expectedPendingCopies := []Copy{
		{
			From:     "builder",
			Src:      []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager"},
			Dest:     "/root/", // destination must contain a trailing slash
			Download: false,
			Chown:    "",
		},
	}
	if !reflect.DeepEqual(mybuilder.PendingCopies, expectedPendingCopies) {
		t.Errorf("Expected %v, got %v\n", expectedPendingCopies, mybuilder.PendingCopies)
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
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
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
	if err := dispatchCopy(&mybuilder2, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
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
	if err := add(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchAdd error: %v", err)
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

func TestDispatchAddChownWithArg(t *testing.T) {
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
	original := "ADD --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := add(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchAdd error: %v", err)
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
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
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
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
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
		Env: []string{"CHOWN_VAL=6731:6731"},
	}

	args := []string{"/go/src/github.com/kubernetes-incubator/service-catalog/controller-manager", "."}
	flagArgs := []string{"--chown=${CHOWN_VAL}"}
	original := "COPY --chown=${CHOWN_VAL} /go/src/github.com/kubernetes-incubator/service-catalog/controller-manager ."
	if err := dispatchCopy(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchCopy error: %v", err)
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
	if err := add(&mybuilder, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchAdd error: %v", err)
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
	if err := add(&mybuilder2, args, nil, flagArgs, original); err != nil {
		t.Errorf("dispatchAdd error: %v", err)
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
