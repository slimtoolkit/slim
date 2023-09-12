#!/bin/bash
sudo apt update -y
sudo apt upgrade -y 
sudo apt install docker -y
sudo systemctl start docker
sudo usermod -aG docker ec2-user
docker [install Jenkins]
docker [mount docker unix on Jenkins]

docker exec [enter Jenkins container]
    apt install nodejs 
    apt install npm
    docker run dslim/slim
    # docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock dslim/slim build your-docker-image-names