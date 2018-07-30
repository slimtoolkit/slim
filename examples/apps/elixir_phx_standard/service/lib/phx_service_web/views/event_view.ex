defmodule PhxServiceWeb.EventView do
  use PhxServiceWeb, :view
  alias PhxServiceWeb.EventView

  def render("index.json", %{events: events}) do
    %{data: render_many(events, EventView, "event.json")}
  end

  def render("show.json", %{event: event}) do
    %{data: render_one(event, EventView, "event.json")}
  end

  def render("event.json", %{event: event}) do
    %{id: event.id,
      type: event.type,
      data: event.data}
  end
end
