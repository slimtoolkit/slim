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
docker build -t ${{github.repository}}:latest
```
### Slim Docker Image
```
slim ${{ github.repository }}:latest -t slim
```
### Push Docker Image to Registry
```
docker image push ${{ github.repository }} --all-tags
```

## References
- [Docker-Slim Github Action](https://github.com/marketplace/actions/docker-slim-github-action)
- [Docker Login Github Action](https://github.com/docker/login-action)
- [Docker Build and Push Github Action](https://github.com/docker/build-push-action)