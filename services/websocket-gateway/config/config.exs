import Config

config :websocket_gateway,
  ecto_repos: []

config :phoenix,
  json_library: Jason

config :websocket_gateway, WebsocketGatewayWeb.Endpoint,
  http: [
    ip: {0, 0, 0, 0},
    port: 8005
  ],
  code_reloader: false,
  watchers: [],
  live_view: [signing_salt: "websocket_gateway_salt"],
  pubsub_server: WebsocketGateway.PubSub,
  secret_key_base: System.get_env("SECRET_KEY_BASE", "dev_secret_key_base_32_character_min"),
  url: [host: System.get_env("HOST", "localhost"), port: 8005]

config :kaffe,
  consumer: [
    bootstrap_servers: System.get_env("KAFKA_BROKERS", "localhost:9092"),
    topics: ["trades.matched"],
    consumer_group: "websocket-gateway",
    consumer_config: [
      fetch_min_bytes: 1000,
      fetch_max_wait_time: 500,
      session_timeout_ms: 20000
    ]
  ]

config :logger,
  level: :info,
  format: "[$level] $message\n"

if Mix.env() == :dev do
  config :logger, :console,
    colors: [enabled: true],
    format: "[$level] $message\n"
end

if Mix.env() == :prod do
  config :logger, level: :info
end
