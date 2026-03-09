defmodule WebsocketGateway.Application do
  use Application

  @impl true
  def start(_type, _args) do
    children = [
      # PubSub for local message broadcasting
      {Phoenix.PubSub, name: WebsocketGateway.PubSub},

      # WebSocket endpoint
      WebsocketGatewayWeb.Endpoint,

      # Redis connection pool
      {Redix, name: :redis, host: redis_host(), port: redis_port()},

      # Kafka consumer for trades.matched events
      WebsocketGateway.KafkaConsumer,

      # Redis publisher for odds updates
      WebsocketGateway.RedisPublisher,

      # Metrics collector
      {Telemetry.Metrics.ConsoleReporter,
       metrics: [
         {Telemetry.Metrics.Counter, name: "websocket_gateway.channel.join",
          description: "Number of channel joins"},
         {Telemetry.Metrics.Counter, name: "websocket_gateway.channel.leave",
          description: "Number of channel leaves"},
         {Telemetry.Metrics.Counter, name: "websocket_gateway.trades.received",
          description: "Number of trades received"},
         {Telemetry.Metrics.Histogram, name: "websocket_gateway.broadcast.latency_ms",
          description: "Broadcast latency in milliseconds", unit: {:millisecond, 1}}
       ]}
    ]

    opts = [strategy: :one_for_one, name: WebsocketGateway.Supervisor]
    Supervisor.start_link(children, opts)
  end

  def config_change(changed, _new, removed) do
    WebsocketGatewayWeb.Endpoint.config_change(changed, removed)
    :ok
  end

  defp redis_host, do: System.get_env("REDIS_HOST", "localhost")
  defp redis_port, do: System.get_env("REDIS_PORT", "6379") |> String.to_integer()
end
