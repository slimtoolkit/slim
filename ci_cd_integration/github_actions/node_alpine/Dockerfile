FROM node:13-alpine

WORKDIR /usr/src/app

COPY . ./

RUN npm install

EXPOSE 3000

CMD ["node", "server.js"]