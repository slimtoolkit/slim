class ApiController < ApplicationController
  def index
  	render json: {:status => 'success', :info => "running", :service => "rails", :stack => 'ruby'}
  end
end
