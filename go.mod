module github.com/docker-slim/docker-slim

go 1.15

require (
	github.com/armon/go-radix v1.0.0
	github.com/bmatcuk/doublestar v1.3.4
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/c-bata/go-prompt v0.2.3
	github.com/c4milo/unpackit v0.0.0-20170704181138-4ed373e9ef1c
	github.com/compose-spec/compose-go v0.0.0-20210916141509-a7e1bc322970
	github.com/docker-slim/go-update v0.0.0-20190422071557-ed40247aff59
	github.com/docker-slim/uiprogress v0.0.0-20190505193231-9d4396e6d40b
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.13.0
	github.com/fsouza/go-dockerclient v1.7.4
	github.com/getkin/kin-openapi v0.76.0
	github.com/ghodss/yaml v1.0.0
	github.com/gocolly/colly/v2 v2.0.1
	github.com/google/go-containerregistry v0.8.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/websocket v1.4.2
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158
	k8s.io/api v0.22.9
	k8s.io/apimachinery v0.22.9
	k8s.io/cli-runtime v0.22.9
	k8s.io/client-go v0.22.9
)

require (
	github.com/PuerkitoBio/goquery v1.5.1 // indirect
	github.com/andybalholm/cascadia v1.2.0 // indirect
	github.com/antchfx/htmlquery v1.2.3 // indirect
	github.com/antchfx/xmlquery v1.3.1 // indirect
	github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8 // indirect
	github.com/docker-slim/uilive v0.0.2 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/google/uuid v1.2.0
	github.com/gosuri/uilive v0.0.3 // indirect
	github.com/hooklift/assert v0.0.0-20170704181755-9d1defd6d214 // indirect
	github.com/klauspost/pgzip v1.2.4 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/pkg/term v0.0.0-20200520122047-c3ffed290a03 // indirect
	github.com/spf13/cobra v1.4.0 // indirect
	github.com/stretchr/testify v1.7.1
	github.com/ulikunitz/xz v0.5.7 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	k8s.io/klog/v2 v2.60.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220328201542-3ee0da9b0b42 // indirect
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect
)

replace github.com/compose-spec/compose-go => ./pkg/third_party/compose-go
