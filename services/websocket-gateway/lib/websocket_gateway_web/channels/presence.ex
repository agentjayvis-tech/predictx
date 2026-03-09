defmodule WebsocketGatewayWeb.Presence do
  use Phoenix.Presence,
    otp_app: :websocket_gateway,
    pubsub_server: WebsocketGateway.PubSub

  def track(socket, user_id, metadata) do
    metadata = Map.put(metadata, :phx_ref, socket.assigns.phx_socket_ref)
    channel_id = socket.channel

    track_presence(
      socket,
      user_id,
      metadata
    )
  end

  def list_users(channel) do
    list(channel)
    |> Enum.map(fn {user_id, %{metas: metas}} ->
      %{
        user_id: user_id,
        count: length(metas),
        metadata: List.first(metas)
      }
    end)
  end

  def get_online_count(channel) do
    list(channel)
    |> Enum.reduce(0, fn {_user_id, %{metas: metas}}, acc ->
      acc + length(metas)
    end)
  end

  def user_online?(channel, user_id) do
    Enum.any?(list(channel), fn {id, _} -> id == user_id end)
  end

  defp track_presence(socket, user_id, metadata) do
    GenServer.call(
      __MODULE__,
      {:track, socket.transport_pid, socket.serializer,
       {socket.channel, user_id, metadata}}
    )
  end
end
