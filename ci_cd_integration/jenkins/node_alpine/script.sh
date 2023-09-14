#!/bin/bash
sudo apt update -y
sudo apt upgrade -y 
sudo apt install docker -y
sudo systemctl start docker
sudo usermod -aG docker ec2-user

# allow port 8080 on EC2 Instance security group; to access jenkins server

#install jenkins in EC2 Instance 
docker run -p 8080:8080 -p 50000:50000 -d -v jenkins_home:/var/jenkins_home jenkins/jenkins:lts

# make docker commands available in jenkins container; expose docker unix socket to Jenkins 
docker stop [jenkins_container_id]
docker run -p 8080:8080 -p 50000:50000 -d \  
-v jenkins_home:/var/jenkins_home \ 
-v /var/run/docker.sock:/var/run/docker.sock \ 
-v $(which docker):/usr/bin/docker jenkins/jenkins:lts

# change permissions of new jenkins container; to execute docker commands
docker exec -u 0 -it [jenkins_container_id] bash
    chmod 666 /var/run/docker.sock 

    # install nodejs/npm in new jenkins container 
    docker exec -u 0 -it [jenkins_container_id] bash
        apt update
        apt install curl 
        curl -sL https://deb.nodesource.com/setup_10.x -o nodesource_setup.sh
        apt install nodejs 
        apt install npm

    # enable nodejs installation on Jenkins interface; Manage Jenkins > Tools 

    # install Slim in new jenkins container 
        docker run dslim/slim