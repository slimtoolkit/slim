%global go_version 1.18.2

Name: slim
Version: 1.40.6
Release: 1%{?dist}
Summary: Slim Toolkit helps you make your containers better, smaller, and secure
License: Apache-2.0
BuildRequires: golang >= %{go_version}
URL: https://github.com/slimtoolkit/slim
Source0: https://github.com/slimtoolkit/slim/archive/refs/tags/%{version}.tar.gz

%define debug_package %{nil}

%prep
%autosetup

%description
Slim Toolkit helps you make your containers better, smaller, and secure

%ifarch x86_64
%define goarch amd64
%endif

%ifarch aarch64
%define goarch arm64
%endif

%ifarch arm
%define goarch arm
%endif

%global slim_version %(git describe --tags --always)
%global slim_revision %(git rev-parse HEAD)
%global slim_buildtime %(date '+%Y-%m-%d_%I:%M:%''S')
%global slim_ldflags -s -w -X github.com/docker-slim/docker-slim/pkg/version.appVersionTag=%{slim_version} -X github.com/docker-slim/docker-slim/pkg/version.appVersionRev=%{slim_revision} -X github.com/docker-slim/docker-slim/pkg/version.appVersionTime=%{slim_buildtime}

%build
export CGO_ENABLED=0
go generate github.com/docker-slim/docker-slim/pkg/appbom
mkdir dist_linux
GOOS=linux GOARCH=%{goarch} go build  -mod=vendor -trimpath -ldflags="%{slim_ldflags}" -a -tags 'netgo osusergo' -o "dist_linux/" ./cmd/slim/...
GOOS=linux GOARCH=%{goarch} go build -mod=vendor -trimpath -ldflags="%{slim_ldflags}" -a -tags 'netgo osusergo' -o "dist_linux/" ./cmd/slim-sensor/...

%install
install -d -m 755 %{buildroot}%{_bindir}
install -d -m 755 %{buildroot}%{_bindir}
install -d -m 755 %{buildroot}/usr/share/doc/slim/
install -d -m 755 %{buildroot}/usr/share/licenses/slim/
install -m 755 dist_linux/%{name} %{buildroot}%{_bindir}
install -m 755 dist_linux/%{name}-sensor %{buildroot}%{_bindir}
install -m 644 README.md %{buildroot}/usr/share/doc/slim/README.md
install -m 644 LICENSE %{buildroot}/usr/share/licenses/slim/LICENSE

%post
%{__ln_s} -f %{_bindir}/%{name} %{_bindir}/docker-slim
chmod a+x %{_bindir}/%{name}
chmod a+x %{_bindir}/%{name}-sensor

%files 
%{_bindir}/%{name}
%{_bindir}/%{name}-sensor
%doc /usr/share/doc/slim/README.md
%license /usr/share/licenses/slim/LICENSE
