FROM ruby:2.2-alpine
 
RUN apk update && apk add build-base ruby-dev && \
		mkdir -p /opt/my/service

COPY service /opt/my/service

WORKDIR /opt/my/service

RUN bundle install

EXPOSE 7000
ENTRYPOINT ["ruby","/opt/my/service/server.rb"]


