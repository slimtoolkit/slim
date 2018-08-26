'use strict';

const Hapi = require('hapi');
const Redis = require('redis');

const rclient = Redis.createClient({host: 'redis', port: 6379});

rclient.on('connect', () => 
{
    console.log('Redis client connected');
});

rclient.on('error', (err) => 
{
    console.log('Redis error - ' + err);
});

const server = Hapi.server(
{
    port: 8000
});

server.route(
{
    method: 'GET',
    path: '/',
    handler: (request, h) =>
    {
        console.log('node service: GET /');

        return new Promise((resolve,reject) => 
        {
            rclient.incr('hapi.call', (err,count) =>
            {
                if (err) 
                {
                    console.log(`redis op error - ${err}`);
                }

                var result =
                {
                    count: count,
                    status: 'success',
                    info: 'yes!!',
                    service: 'node.compose',
                    version: process.version
                };

                resolve(result);
            });
        });
    }
});

const init = async () => 
{
    await server.start();
    console.log(`node service: ${server.info.uri}`);
};

process.on('unhandledRejection', (err) =>
{
    console.log(err);
    process.exit(1);
});

init();
