FROM node:8.11

WORKDIR /opt/my/service

COPY service /opt/my/service

RUN yarn install

EXPOSE 8000
ENTRYPOINT ["node","/opt/my/service/server.js"]


