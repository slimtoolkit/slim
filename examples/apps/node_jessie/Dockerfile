FROM debian:jessie

RUN apt-get update && \
	apt-get install -y curl && \
	curl -sL https://deb.nodesource.com/setup_4.x | bash - && \
	apt-get install -y build-essential nodejs && \
	mkdir -p /opt/my/service

COPY service /opt/my/service

WORKDIR /opt/my/service

RUN npm install

EXPOSE 8000
ENTRYPOINT ["/opt/my/service/docker-entrypoint.sh"]


