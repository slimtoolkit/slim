
var Hapi = require('hapi');

var server = new Hapi.Server();
server.connection(
{
    port: 8000 
});

server.route(
{
    method: 'GET',
    path:'/', 
    handler: function (request, reply) 
    {
       console.log('demo service: GET /');
       reply({status: 'success', info: 'yes!!!'});
    }
});


server.start(function()
{
	console.log('demo service: running at:', server.info.uri);
});
