FROM ruby:2.5.3

WORKDIR /opt/my/service
COPY service /opt/my/service

RUN bundle install

EXPOSE 3333
ENTRYPOINT ["rails","server","--binding=0.0.0.0","--port=3333","--environment=development"]


