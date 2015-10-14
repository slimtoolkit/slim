FROM ubuntu:14.04

RUN apt-get update && \
		apt-get -y install software-properties-common && \
		add-apt-repository -y ppa:webupd8team/java && \
		apt-get update && \
		echo oracle-java7-installer shared/accepted-oracle-license-v1-1 select true | /usr/bin/debconf-set-selections && \
		apt-get -y install oracle-java7-installer && \
		update-java-alternatives -s java-7-oracle && \
		mkdir -p /opt/my/service

COPY target/java-service-0.0.1.jar /opt/my/service/java-service.jar
COPY service.yml /opt/my/service/service.yml
COPY service.keystore /opt/my/service/service.keystore

WORKDIR /opt/my/service

EXPOSE 8080
ENTRYPOINT ["java","-jar","/opt/my/service/java-service.jar","server","service.yml"]


