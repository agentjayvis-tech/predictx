defmodule WebsocketGateway.KafkaConsumer do
  @moduledoc """
  Consumes trade.matched events from Kafka and publishes to Redis Pub/Sub.
  """

  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, %{}, name: __MODULE__)
  end

  @impl true
  def init(state) do
    # Start Kafka consumer in a separate process
    send(self(), :start_consumer)
    {:ok, state}
  end

  @impl true
  def handle_info(:start_consumer, state) do
    {:ok, _pid} = start_kafka_consumer()
    {:noreply, state}
  end

  defp start_kafka_consumer do
    {:ok, _pid} =
      Task.start_link(fn ->
        Logger.info("Starting Kafka consumer for trades.matched")
        consume_trades()
      end)
  end

  defp consume_trades do
    {:ok, pid} =
      :brod.start_link_client([
        localhost: 9092
      ])

    :brod.consume_messages(pid, [
      {"trades.matched", 0}
    ])

    loop(pid)
  end

  defp loop(pid) do
    receive do
      msg -> handle_message(msg)
    end

    loop(pid)
  end

  defp handle_message({:messages, _partition, messages}) when is_list(messages) do
    Enum.each(messages, fn msg ->
      case Jason.decode(msg.value) do
        {:ok, trade} ->
          broadcast_odds_update(trade)
          :telemetry.execute([:websocket_gateway, :trades, :received], %{count: 1})

        {:error, reason} ->
          Logger.error("Failed to decode trade message: #{inspect(reason)}")
      end
    end)
  end

  defp handle_message(_), do: :ok

  defp broadcast_odds_update(trade) do
    market_id = trade["market_id"]
    timestamp = System.monotonic_time(:millisecond)

    odds_update = %{
      market_id: market_id,
      match_type: trade["match_type"],
      price_minor: trade["price_minor"],
      quantity: trade["quantity"],
      buyer_id: trade["buyer_id"],
      seller_id: trade["seller_id"],
      timestamp: timestamp,
      latency_ms: 0
    }

    # Publish to Redis Pub/Sub for local fan-out
    publish_to_redis(market_id, odds_update)

    # Broadcast to Phoenix PubSub
    broadcast_to_pubsub(market_id, odds_update)
  end

  defp publish_to_redis(market_id, odds_update) do
    key = "market:#{market_id}:odds"

    case Redix.command(:redis, [
      "PUBLISH",
      key,
      Jason.encode!(odds_update)
    ]) do
      {:ok, _subscribers} ->
        :ok

      {:error, reason} ->
        Logger.error("Failed to publish to Redis: #{inspect(reason)}")
    end
  end

  defp broadcast_to_pubsub(market_id, odds_update) do
    Phoenix.PubSub.broadcast(
      WebsocketGateway.PubSub,
      "market:#{market_id}",
      {:odds_updated, odds_update}
    )
  end
end
