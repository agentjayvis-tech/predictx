defmodule WebsocketGateway.MixProject do
  use Mix.Project

  def project do
    [
      app: :websocket_gateway,
      version: "0.1.0",
      elixir: "~> 1.14",
      start_permanent: Mix.env() == :prod,
      deps: deps(),
      aliases: aliases()
    ]
  end

  def application do
    [
      extra_applications: [:logger],
      mod: {WebsocketGateway.Application, []}
    ]
  end

  defp deps do
    [
      {:phoenix, "~> 1.7"},
      {:phoenix_live_view, "~> 0.20"},
      {:plug_cowboy, "~> 2.6"},
      {:jason, "~> 1.4"},
      {:joken, "~> 2.5"},
      {:brod, "~> 3.19"},
      {:redix, "~> 1.2"},
      {:telemetry, "~> 1.2"},
      {:telemetry_metrics, "~> 0.6"},
      {:telemetry_poller, "~> 1.0"},
      {:credo, "~> 1.7", only: [:dev, :test]},
      {:ex_doc, "~> 0.31", only: :dev}
    ]
  end

  defp aliases do
    [
      test: "test"
    ]
  end
end
