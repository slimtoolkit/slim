defmodule PhxServiceWeb.EventController do
  use PhxServiceWeb, :controller

  action_fallback PhxServiceWeb.FallbackController

  def index(conn, _params) do
    events = [%{id: 1, type: "status", data: "up"},%{id: 2, type: "start", data: "12/12/2018:123456"}]
    render(conn, "index.json", events: events)
  end

  def show(conn, %{"id" => id}) do
    render(conn, "show.json", event: %{id: id, type: "info", data: "Elixir phoenix"})
  end
end
