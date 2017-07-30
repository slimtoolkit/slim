require 'bundler/setup'
require 'sinatra'
require 'json'

set :bind, '0.0.0.0'
set :port, 7000

get '/', :provides => :json do
  {:status => 'success',:info => 'yes!!!', :service => 'ruby'}.to_json
end
