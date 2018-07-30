FROM elixir:1.6

RUN mix local.hex --force && \
    mix local.rebar --force

COPY service /opt/my/service
WORKDIR /opt/my/service

RUN mix do deps.get, deps.compile, compile

EXPOSE 16000
CMD ["mix", "phoenix.server"]
