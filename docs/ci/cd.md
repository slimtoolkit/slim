# CI/CD Integrations (compact)

This file collects practical guidance and examples for integrating SlimToolkit into CI/CD pipelines (GitHub Actions, GitLab CI, Jenkins, Cloud Build, etc.).

Quick checklist
- Inputs: built image tag (or Dockerfile + context).
- Outputs: slimmed image tag (recommended: `<name>:slim-<build-id>`) or artifacts directory with `Dockerfile` + `files.tar`.
- Key failure modes: no Docker socket, restricted privileges (seccomp/AppArmor), registry auth failures.

Recommended flow
1. Build the fat image on the CI runner (don't push yet).
2. Run Slim on the runner (mount the Docker socket or configure DOCKER_HOST) so it can access the local image.
3. Tag and push the slim image to your registry.

Important flags and env
- `--target <image:tag>`: image to slim.
- `--tag <tag>`: tag for the slim output image.
- `DSLIM_HTTP_PROBE` / `--http-probe`: control HTTP probing (set `DSLIM_HTTP_PROBE=false` in CI if probes are flaky).
- `--continue-after`: use `timeout`, `signal`, or `exec` to automate completion in CI.
- `--sensor-ipc-mode` and `--sensor-ipc-endpoint`: tune sensor communication in constrained environments.

GitHub Actions (runner builds image, runs Slim, pushes)

Example snippet:

```yaml
name: Build and Slim
on: [push]
jobs:
  slim:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Build image (local)
        uses: docker/build-push-action@v4
        with:
          push: false
          tags: myapp:${{ github.run_number }}

      - name: Slim the image
        env:
          DSLIM_HTTP_PROBE: false
        run: |
          docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
            dslim/slim build --target myapp:${{ github.run_number }} --tag myapp:slim-${{ github.run_number }} --continue-after=timeout

      - name: Login and push
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.REGISTRY_USER }}
          password: ${{ secrets.REGISTRY_TOKEN }}

      - run: docker push myapp:slim-${{ github.run_number }}
```

Notes:
- Mounting `/var/run/docker.sock` gives Slim access to the runner's Docker daemon. If using `docker:dind` or a remote daemon, set `DOCKER_HOST` and copy TLS certs as needed.
- If HTTP probing is unreliable in CI, disable it and use `--exec` or `--continue-after=signal` with your test runner.

GitLab CI (dind) minimal example

```yaml
variables:
  DOCKER_HOST: tcp://docker:2375
  DOCKER_TLS_CERTDIR: ""
services:
  - name: docker:dind
    command: ["--host=tcp://0.0.0.0:2375"]

build:
  script:
    - docker build -t myapp:$CI_PIPELINE_IID .

slim:
  script:
    - docker run --rm -e DSLIM_HTTP_PROBE=false -v /var/run/docker.sock:/var/run/docker.sock \
      dslim/slim build --target myapp:$CI_PIPELINE_IID --tag myapp:slim-$CI_PIPELINE_IID --continue-after=timeout

push:
  script:
    - echo "$CI_REGISTRY_PASSWORD" | docker login -u "$CI_REGISTRY_USER" --password-stdin $CI_REGISTRY
    - docker push myapp:slim-$CI_PIPELINE_IID
```

Jenkins (quick snippet)

```groovy
stage('Build and Slim') {
  steps {
    sh 'docker build -t myapp:$BUILD_NUMBER .'
    sh "docker run --rm -v /var/run/docker.sock:/var/run/docker.sock dslim/slim build --target myapp:$BUILD_NUMBER --tag myapp:slim-$BUILD_NUMBER --continue-after=timeout"
  }
}
```

Containerized CI gotchas
- Missing Docker socket: mount `/var/run/docker.sock` or set `DOCKER_HOST`.
- Privileges: Slim may need elevated capabilities to generate seccomp/AppArmor profiles; consider a privileged runner or disable profile generation.
- File flags: flags that take file paths (e.g., `--include-path-file`) require those files to be mounted into the Slim container.

Troubleshooting checklist
- Sensor communication issues: try `--sensor-ipc-mode=proxy` or set `--sensor-ipc-endpoint` explicitly.
- Probes failing: set `DSLIM_HTTP_PROBE=false` and drive tests with `--exec` or `--continue-after=signal`.
- Permission errors: ensure the runner user can access the Docker socket and mounted files.

If you'd like, I can further shorten any example or add one specific to your CI provider.
