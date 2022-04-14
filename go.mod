module github.com/docker-slim/docker-slim

go 1.13

require (
	github.com/PuerkitoBio/goquery v1.5.1 // indirect
	github.com/andybalholm/cascadia v1.2.0 // indirect
	github.com/antchfx/htmlquery v1.2.3 // indirect
	github.com/antchfx/xmlquery v1.2.4 // indirect
	github.com/antchfx/xpath v1.1.8 // indirect
	github.com/armon/go-radix v1.0.0
	github.com/bitly/go-hostpool v0.1.0 // indirect
	github.com/bmatcuk/doublestar v1.3.4
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8 // indirect
	github.com/c-bata/go-prompt v0.2.3
	github.com/c4milo/unpackit v0.0.0-20170704181138-4ed373e9ef1c
	github.com/compose-spec/compose-go v1.2.1
	github.com/denisenkom/go-mssqldb v0.12.0 // indirect
	github.com/docker-slim/go-update v0.0.0-20190422071557-ed40247aff59
	github.com/docker-slim/uilive v0.0.2 // indirect
	github.com/docker-slim/uiprogress v0.0.0-20190505193231-9d4396e6d40b
	github.com/docker/buildx v0.8.2
	github.com/docker/cli v20.10.12+incompatible
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/erikstmartin/go-testdb v0.0.0-20160219214506-8d10e4a1bae5 // indirect
	github.com/fatih/color v1.13.0
	github.com/fsouza/go-dockerclient v1.7.4
	github.com/getkin/kin-openapi v0.76.0
	github.com/ghodss/yaml v1.0.0
	github.com/gocolly/colly/v2 v2.0.1
	github.com/google/go-containerregistry v0.8.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/websocket v1.4.2
	github.com/gosuri/uilive v0.0.3 // indirect
	github.com/hooklift/assert v0.0.0-20170704181755-9d1defd6d214 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/pgzip v1.2.4 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/moby/buildkit v0.10.1-0.20220403220257-10e6f94bf90d
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/opencontainers/image-spec v1.0.2-0.20211117181255-693428a734f5
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v0.0.0-20200520122047-c3ffed290a03 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9
)

replace github.com/compose-spec/compose-go => ./pkg/third_party/compose-go

replace (
	// These pins are needed until this PR is merged:
	// https://github.com/tonistiigi/fsutil/pull/105
	github.com/docker/cli => github.com/docker/cli v20.10.3-0.20220226190722-8667ccd1124c+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.3-0.20220224222438-c78f6963a1c0+incompatible
	// TODO: test the upgrade (v0.72.0) and remove this pin.
	github.com/getkin/kin-openapi => github.com/getkin/kin-openapi v0.19.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.8
)
