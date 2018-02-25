FROM php:7.0-cli
COPY src/ /usr/app
WORKDIR /usr/app
EXPOSE 8000

CMD ["php","-S","0.0.0.0:8000","./index.php"]