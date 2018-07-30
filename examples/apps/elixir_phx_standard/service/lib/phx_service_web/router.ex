defmodule PhxServiceWeb.Router do
  use PhxServiceWeb, :router

  pipeline :api do
    plug :accepts, ["json"]
  end

  scope "/", PhxServiceWeb do
    pipe_through :api

    resources "/", EventController, only: [:index, :show]
  end

  scope "/api", PhxServiceWeb do
    pipe_through :api

    resources "/events", EventController, only: [:index, :show]
  end
end
