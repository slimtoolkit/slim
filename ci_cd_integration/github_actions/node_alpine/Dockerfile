FROM node:13-alpine

RUN mkdir -p /usr/app

COPY . /usr/app/

WORKDIR /usr/app

EXPOSE 3000

RUN npm install

CMD ["node", "server.js"]