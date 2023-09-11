## Docker-Slim integration for a basic Nodejs app in a Github Actions CI [workflow](https://github.com/TomiwaAribisala-git/slim/blob/slim-ci_cd_integration/.github/workflows/node_alpine.yml)

### Test Nodejs app 
```
npm install
```
### Install dependencies 
```
npm run test 
```
### Build app artifact   
```
npm pack
``` 
### Build Docker Image
```
docker build -t node_alpine:${{github.run_number}}
```
### Slim Docker Image
```
slim node_alpine:${{github.run_number}} -t slim-${{github.run_number}}
```
### Docker Login
```
docker login -u ${{ secrets.DOCKERHUB_USERNAME }} -p ${{ secrets.DOCKERHUB_TOKEN }}
```
### Tag Image
```
docker tag node_alpine:slim-${{github.run_number}} ${{ secrets.DOCKERHUB_USERNAME }}/node_alpine:slim-${{github.run_number}}
```
### Push Docker Image to Registry
```
docker push node_alpine:slim-${{github.run_number}} 
```

## References
- [Docker Docs: Github Actions](https://docs.docker.com/build/ci/github-actions/)
- [Docker Hub](https://hub.docker.com/)
- [Docker-Slim Github Action](https://github.com/marketplace/actions/docker-slim-github-action)
- [Docker Login Github Action](https://github.com/docker/login-action)
- [Docker Build and Push Github Action](https://github.com/docker/build-push-action)