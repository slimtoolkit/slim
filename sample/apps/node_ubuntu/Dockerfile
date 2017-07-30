FROM ubuntu:14.04

RUN apt-get update && \
		apt-get install -y curl software-properties-common python-software-properties && \
		add-apt-repository ppa:chris-lea/node.js && \
		apt-get update && \
		apt-get install -y build-essential \
		nodejs && \
		mkdir -p /opt/my/service

COPY service /opt/my/service

WORKDIR /opt/my/service

RUN npm install

EXPOSE 8000
ENTRYPOINT ["node","/opt/my/service/server.js"]


