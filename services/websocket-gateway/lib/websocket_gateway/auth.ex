defmodule WebsocketGateway.Auth do
  @moduledoc """
  JWT token verification for WebSocket connections.
  """

  def verify_token(token) when is_binary(token) do
    secret = System.get_env("JWT_SECRET", "dev_secret")

    case Joken.verify(token, Joken.default_claims(secret)) do
      {:ok, claims} ->
        {:ok, claims}

      {:error, reason} ->
        {:error, reason}
    end
  end

  def verify_token(_), do: {:error, :invalid_token}

  def create_token(user_id, user_name) do
    secret = System.get_env("JWT_SECRET", "dev_secret")

    claims = %{
      "sub" => user_id,
      "name" => user_name,
      "iat" => System.unix_time(:second),
      "exp" => System.unix_time(:second) + 86400
    }

    case Joken.encode(claims, secret) do
      {:ok, token} -> {:ok, token}
      error -> error
    end
  end
end
