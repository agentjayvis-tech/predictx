defmodule WebsocketGatewayTest do
  use ExUnit.Case
  doctest WebsocketGateway

  test "application starts" do
    assert {:ok, _pid} = Application.ensure_all_started(:websocket_gateway)
  end
end
