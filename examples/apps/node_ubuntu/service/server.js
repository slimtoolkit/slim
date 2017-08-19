
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
       console.log('node service: GET /');
       reply({status: 'success', info: 'yes!!!', 'service': 'node'});
    }
});


server.start(function()
{
	console.log('node service: ', server.info.uri);
});
