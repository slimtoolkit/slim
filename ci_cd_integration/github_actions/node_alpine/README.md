## Docker-Slim integration for a Nodejs application in a Github Actions CI/CD [workflow]()

### Test Nodejs app 
```
npm run test
```
### Install dependencies 
```
npm install 
```
### Build app artifact 
```
npm pack
``` 
### Build Docker Image
```
docker build -t node_alpine:latest
```
### Slim Docker Image
```
slim node_alpine:latest -t node_alpine_slim:${{github.run_number}}
```
### Push Slim Docker Image to Registry
```
docker image push ${{ secrets.DOCKERHUB_USERNAME }}/node_alpine_slim:${{github.run_number}}
```

## References
- [Docker-Slim Github Action](https://github.com/marketplace/actions/docker-slim-github-action)
- [Docker Login Github Action](https://github.com/docker/login-action)
- [Docker Build and Push Github Action](https://github.com/docker/build-push-action)