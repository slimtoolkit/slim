module github.com/docker-slim/docker-slim

go 1.13

require (
	github.com/PuerkitoBio/goquery v1.5.1 // indirect
	github.com/andybalholm/cascadia v1.2.0 // indirect
	github.com/antchfx/htmlquery v1.2.3 // indirect
	github.com/antchfx/xmlquery v1.2.4 // indirect
	github.com/antchfx/xpath v1.1.8 // indirect
	github.com/armon/go-radix v1.0.0
	github.com/bmatcuk/doublestar v1.3.4
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8 // indirect
	github.com/c-bata/go-prompt v0.2.3
	github.com/c4milo/unpackit v0.0.0-20170704181138-4ed373e9ef1c
	github.com/compose-spec/compose-go v0.0.0-20210916141509-a7e1bc322970
	github.com/docker-slim/go-update v0.0.0-20190422071557-ed40247aff59
	github.com/docker-slim/uilive v0.0.2 // indirect
	github.com/docker-slim/uiprogress v0.0.0-20190505193231-9d4396e6d40b
	github.com/docker/docker v20.10.12+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.13.0
	github.com/fsouza/go-dockerclient v1.7.4
	github.com/getkin/kin-openapi v0.19.0
	github.com/ghodss/yaml v1.0.0
	github.com/gocolly/colly/v2 v2.0.1
	github.com/google/go-containerregistry v0.8.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/gorilla/websocket v1.4.2
	github.com/gosuri/uilive v0.0.3 // indirect
	github.com/hooklift/assert v0.0.0-20170704181755-9d1defd6d214 // indirect
	github.com/klauspost/pgzip v1.2.4 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mattn/go-tty v0.0.3 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v0.0.0-20200520122047-c3ffed290a03 // indirect
	github.com/robertkrimen/otto v0.0.0-20211024170158-b87d35c0b86f
	github.com/sirupsen/logrus v1.8.1
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	github.com/ulikunitz/xz v0.5.7 // indirect
	github.com/urfave/cli/v2 v2.3.0
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e
)

replace github.com/compose-spec/compose-go => ./pkg/third_party/compose-go
