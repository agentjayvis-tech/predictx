defmodule WebsocketGateway.RedisPublisher do
  @moduledoc """
  Subscribes to Redis Pub/Sub for odds updates and broadcasts to Phoenix channels.
  Handles ~50ms latency requirement.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, %{}, name: __MODULE__)
  end

  @impl true
  def init(state) do
    send(self(), :subscribe)
    {:ok, state}
  end

  @impl true
  def handle_info(:subscribe, state) do
    {:ok, subscriber_pid} = Redix.PubSub.new_connection()

    # Subscribe to all market odds channels
    Redix.PubSub.subscribe(subscriber_pid, "market:*:odds", fn
      {:message, _topic, payload} ->
        handle_odds_update(payload)

      {:pmessage, _pattern, _topic, payload} ->
        handle_odds_update(payload)

      error ->
        Logger.error("Redis subscription error: #{inspect(error)}")
    end)

    {:noreply, Map.put(state, :subscriber_pid, subscriber_pid)}
  end

  defp handle_odds_update(payload) do
    start_time = System.monotonic_time(:millisecond)

    case Jason.decode(payload) do
      {:ok, odds} ->
        market_id = odds["market_id"]
        latency = System.monotonic_time(:millisecond) - start_time

        # Broadcast to all subscribers in this market
        Phoenix.PubSub.broadcast(
          WebsocketGateway.PubSub,
          "market:#{market_id}",
          {"odds:updated",
           %{
             market_id: market_id,
             match_type: odds["match_type"],
             price_minor: odds["price_minor"],
             quantity: odds["quantity"],
             buyer_id: odds["buyer_id"],
             seller_id: odds["seller_id"],
             timestamp: odds["timestamp"],
             gateway_latency_ms: latency
           }}
        )

        # Record latency metric
        :telemetry.execute(
          [:websocket_gateway, :broadcast, :latency_ms],
          %{latency: latency}
        )

      {:error, reason} ->
        Logger.error("Failed to decode odds update: #{inspect(reason)}")
    end
  end
end
