defmodule WebsocketGatewayWeb.PresenceChannel do
  use Phoenix.Channel

  alias WebsocketGatewayWeb.Presence

  def join("presence:" <> channel_id, _params, socket) do
    send(self(), :after_join)

    {:ok,
     socket
     |> assign(:channel_id, channel_id)
     |> assign(:presence_ref, nil)}
  end

  def handle_info(:after_join, socket) do
    {:ok, _ref} =
      Presence.track(socket, socket.assigns.user_id, %{
        user_name: socket.assigns.user_name,
        online_at: System.os_time(:millisecond)
      })

    push(socket, "presence_state", Presence.list(socket.channel))
    {:noreply, socket}
  end

  def handle_info(%Phoenix.Socket.Broadcast{event: "presence_diff", payload: diff}, socket) do
    push(socket, "presence_diff", diff)
    {:noreply, socket}
  end

  def terminate(_reason, _socket) do
    :ok
  end
end
