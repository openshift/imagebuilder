module github.com/openshift/imagebuilder

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/containerd/containerd v1.3.0
	github.com/containers/storage v0.0.0-20181207174215-bf48aa83089d
	github.com/docker/docker v1.4.2-0.20170829193243-b68221c37ee5
	github.com/docker/go-connections v0.4.1-0.20180821093606-97c2040d34df // indirect
	github.com/docker/go-units v0.3.4-0.20181030082039-2fb04c6466a5 // indirect
	github.com/fsouza/go-dockerclient v1.7.3
	github.com/gogo/protobuf v1.2.2-0.20190306082329-c5a62797aee0 // indirect
	github.com/golang/protobuf v1.2.0 // indirect
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1.0.20190228220655-ac19fd6e7483 // indirect
	github.com/opencontainers/image-spec v1.0.2-0.20190306222905-243ea084a444 // indirect
	github.com/opencontainers/runc v1.0.0-rc6.0.20190305074555-923a8f8a9a07 // indirect
	github.com/pkg/errors v0.8.2-0.20190227000051-27936f6d90f9
	github.com/pquerna/ffjson v0.0.0-20171002144729-d49c2bc1aa13 // indirect
	github.com/sirupsen/logrus v1.3.1-0.20190306131408-d7b6bf5e4d26 // indirect
	github.com/stretchr/testify v1.6.1
	golang.org/x/crypto v0.0.0-20190103213133-ff983b9c42bc // indirect
	golang.org/x/net v0.0.0-20190107210223-45ffb0cd1ba0 // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a // indirect
	golang.org/x/sys v0.0.0-20190108104531-7fbe1cd0fcc2 // indirect
	google.golang.org/genproto v0.0.0-20170523043604-d80a6e20e776 // indirect
	google.golang.org/grpc v1.14.0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/klog v0.2.1-0.20190306015804-8e90cee79f82
)

replace github.com/fsouza/go-dockerclient => github.com/openshift/go-dockerclient v0.0.0-20181016170459-ff9568be2219
