defmodule WebsocketGatewayWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :websocket_gateway

  socket "/socket", WebsocketGatewayWeb.UserSocket,
    websocket: [timeout: 45_000, max_frame_size: 16_000]

  def init(_key, config) do
    {:ok, config}
  end
end
