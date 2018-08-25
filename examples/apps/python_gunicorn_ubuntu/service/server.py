import falcon 
import json
import uuid
import arrow
from wsgiref import simple_server
import platform

class ApiRoot: 
    def on_get(self, req, resp):
        print 'GET /'
        now = arrow.utcnow()

        data = {
        'status': 'success',
        'info': 'yes!!!',
        'service': 'python',
        'id': uuid.uuid4().hex,
        'time': now.isoformat(),
        'version': platform.python_version()}

        resp.status = falcon.HTTP_200
        resp.body = json.dumps(data)

def create_api():
    api = falcon.API()
    api.add_route('/', ApiRoot())
    return api

if __name__ == "__main__":
  try:
    api = create_api()
    
    server = simple_server.make_server('0.0.0.0',9000,api)
    server.serve_forever()
  except KeyboardInterrupt:
    pass

