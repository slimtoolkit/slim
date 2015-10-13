FROM ubuntu:14.04

RUN apt-get update && \
		apt-get -y install software-properties-common build-essential && \
		apt-add-repository ppa:brightbox/ruby-ng && \
		apt-get update && \
		apt-get -y install ruby2.2 ruby2.2-dev && \
		gem install bundler && \
		mkdir -p /opt/my/service

COPY service /opt/my/service

WORKDIR /opt/my/service

RUN bundle install

EXPOSE 7000
ENTRYPOINT ["ruby","/opt/my/service/server.rb"]


