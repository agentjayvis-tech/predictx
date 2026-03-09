defmodule WebsocketGatewayWeb.UserSocket do
  use Phoenix.Socket

  channel "market:*", WebsocketGatewayWeb.MarketChannel
  channel "presence:*", WebsocketGatewayWeb.PresenceChannel

  def connect(params, socket, _connect_info) do
    case WebsocketGateway.Auth.verify_token(params["token"]) do
      {:ok, claims} ->
        {:ok,
         socket
         |> assign(:user_id, claims["sub"])
         |> assign(:user_name, claims["name"])
         |> assign(:connected_at, System.monotonic_time(:millisecond))}

      {:error, _reason} ->
        :error
    end
  end

  def id(socket), do: "user:#{socket.assigns.user_id}"
end
