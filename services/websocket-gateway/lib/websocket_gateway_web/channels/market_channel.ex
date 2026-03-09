defmodule WebsocketGatewayWeb.MarketChannel do
  use Phoenix.Channel

  alias WebsocketGatewayWeb.Presence

  intercept(["odds:updated", "chat:new_message", "presence:diff"])

  def join("market:" <> market_id, %{}, socket) do
    # Verify market exists (in production, call market service)
    if market_exists?(market_id) do
      send(self(), :after_join)

      {:ok,
       socket
       |> assign(:market_id, market_id)
       |> assign(:joined_at, System.monotonic_time(:millisecond))}
    else
      {:error, %{reason: "Market not found"}}
    end
  end

  def join(_, _, _socket) do
    {:error, %{reason: "Invalid channel"}}
  end

  def handle_info(:after_join, socket) do
    # Track user presence in market
    Presence.track(socket, socket.assigns.user_id, %{
      user_name: socket.assigns.user_name,
      joined_at: socket.assigns.joined_at,
      online_at: System.os_time(:millisecond)
    })

    # Emit metrics
    :telemetry.execute([:websocket_gateway, :channel, :join], %{count: 1})

    {:noreply, socket}
  end

  def handle_in("chat", %{"message" => message}, socket) when is_binary(message) do
    market_id = socket.assigns.market_id
    user_id = socket.assigns.user_id
    user_name = socket.assigns.user_name

    chat_msg = %{
      market_id: market_id,
      user_id: user_id,
      user_name: user_name,
      message: String.slice(message, 0..500),
      timestamp: System.unix_time(:millisecond)
    }

    # Broadcast to all subscribers in this market
    broadcast(socket, "chat:new_message", chat_msg)

    # Persist to Redis (optional, for history)
    persist_chat_message(market_id, chat_msg)

    {:noreply, socket}
  end

  def handle_in("chat", _, socket) do
    {:reply, {:error, %{reason: "Invalid message"}}, socket}
  end

  def handle_out("odds:updated", payload, socket) do
    push(socket, "odds:updated", payload)
    {:noreply, socket}
  end

  def handle_out("chat:new_message", payload, socket) do
    push(socket, "chat:new_message", payload)
    {:noreply, socket}
  end

  def handle_out("presence:diff", payload, socket) do
    push(socket, "presence:diff", payload)
    {:noreply, socket}
  end

  def terminate(_reason, socket) do
    :telemetry.execute([:websocket_gateway, :channel, :leave], %{count: 1})
    :ok
  end

  # Helpers
  defp market_exists?(market_id) do
    # In production, call Market Service gRPC/HTTP
    String.length(market_id) > 0
  end

  defp persist_chat_message(market_id, msg) do
    key = "market:#{market_id}:chat"

    case Redix.command(:redis, [
      "LPUSH",
      key,
      Jason.encode!(msg)
    ]) do
      {:ok, _} ->
        # Keep last 100 messages per market
        Redix.command(:redis, ["LTRIM", key, 0, 99])

      {:error, reason} ->
        Logger.error("Failed to persist chat message: #{inspect(reason)}")
    end
  end
end
