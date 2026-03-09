from pydantic_settings import BaseSettings, SettingsConfigDict
from functools import lru_cache


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

    # Database
    database_url: str = "postgresql+asyncpg://postgres:postgres@localhost:5432/predictx"

    # Redis / Celery
    redis_url: str = "redis://localhost:6379/0"
    celery_broker_url: str = "redis://localhost:6379/1"
    celery_result_backend: str = "redis://localhost:6379/2"

    # Kafka
    kafka_bootstrap_servers: str = "localhost:9092"

    # External APIs
    api_football_key: str = ""
    api_football_base_url: str = "https://v3.football.api-sports.io"
    openweathermap_key: str = ""
    openweathermap_base_url: str = "https://api.openweathermap.org/data/2.5"

    # Anthropic
    anthropic_api_key: str = ""
    anthropic_model: str = "claude-haiku-4-5-20251001"

    # Resolution settings
    resolution_check_interval_seconds: int = 60
    dispute_window_minutes: int = 15
    ai_confidence_threshold: float = 0.95   # escalate to human if below this
    insurance_fund_fee_bps: int = 50        # 0.5% of each market pool

    # Judge settings
    judge_decision_deadline_hours: int = 48


@lru_cache
def get_settings() -> Settings:
    return Settings()
