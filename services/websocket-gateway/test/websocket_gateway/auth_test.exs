defmodule WebsocketGateway.AuthTest do
  use ExUnit.Case

  alias WebsocketGateway.Auth

  describe "create_token/2" do
    test "creates valid JWT token" do
      {:ok, token} = Auth.create_token("user123", "John Doe")

      assert is_binary(token)
      assert String.length(token) > 0
    end
  end

  describe "verify_token/1" do
    test "verifies valid token" do
      {:ok, token} = Auth.create_token("user123", "John Doe")
      {:ok, claims} = Auth.verify_token(token)

      assert claims["sub"] == "user123"
      assert claims["name"] == "John Doe"
    end

    test "rejects invalid token" do
      {:error, _reason} = Auth.verify_token("invalid_token")
    end

    test "rejects nil token" do
      {:error, :invalid_token} = Auth.verify_token(nil)
    end
  end
end
