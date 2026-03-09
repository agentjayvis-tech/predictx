"""
Automated market resolvers.

Each resolver accepts a MarketContext (market metadata + resolution criteria)
and returns a ResolutionProposal or None if data is not yet available.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from datetime import datetime
from typing import Any
from uuid import UUID

import httpx

from .config import get_settings

logger = logging.getLogger(__name__)
settings = get_settings()


@dataclass
class MarketContext:
    market_id: UUID
    question: str
    resolution_criteria: str
    category: str           # "sports" | "weather" | "finance" | ...
    metadata: dict[str, Any]    # category-specific hints (fixture_id, city, etc.)
    closes_at: datetime


@dataclass
class ResolutionProposal:
    market_id: UUID
    outcome: str            # "YES" | "NO" | "VOID"
    source: str             # resolver name
    evidence: str           # JSON-serialisable evidence string
    confidence: float       # 0.0–1.0 (pre-AI; AI validator may override)


# ---------------------------------------------------------------------------
# Sports resolver — API-Football
# ---------------------------------------------------------------------------

class SportsResolver:
    """
    Resolves sports prediction markets using API-Football.

    Expected market metadata keys:
      fixture_id  : int       API-Football fixture ID
      team_home   : str       home team name (fallback search)
      team_away   : str       away team name
      condition   : str       "home_win" | "away_win" | "draw" | "btts" | "over_X.5"
    """

    BASE_URL = settings.api_football_base_url
    HEADERS = {
        "x-apisports-key": settings.api_football_key,
        "x-rapidapi-host": "v3.football.api-sports.io",
    }

    async def resolve(self, ctx: MarketContext) -> ResolutionProposal | None:
        fixture_id = ctx.metadata.get("fixture_id")
        if not fixture_id:
            logger.warning("SportsResolver: no fixture_id for market %s", ctx.market_id)
            return None

        async with httpx.AsyncClient(timeout=10) as client:
            resp = await client.get(
                f"{self.BASE_URL}/fixtures",
                headers=self.HEADERS,
                params={"id": fixture_id},
            )
            resp.raise_for_status()
            data = resp.json()

        fixtures = data.get("response", [])
        if not fixtures:
            return None

        fixture = fixtures[0]
        status_short = fixture["fixture"]["status"]["short"]

        # Only resolve if match is finished
        if status_short not in ("FT", "AET", "PEN"):
            logger.debug("Fixture %s not finished yet (status=%s)", fixture_id, status_short)
            return None

        goals = fixture["goals"]
        home_goals: int = goals["home"] or 0
        away_goals: int = goals["away"] or 0
        condition: str = ctx.metadata.get("condition", "home_win")

        outcome = self._evaluate_condition(condition, home_goals, away_goals, fixture)
        evidence = json.dumps({
            "fixture_id": fixture_id,
            "status": status_short,
            "home_goals": home_goals,
            "away_goals": away_goals,
            "condition": condition,
        })

        return ResolutionProposal(
            market_id=ctx.market_id,
            outcome=outcome,
            source="sports_api",
            evidence=evidence,
            confidence=0.98,    # live score data is authoritative
        )

    def _evaluate_condition(
        self, condition: str, home: int, away: int, fixture: dict
    ) -> str:
        if condition == "home_win":
            return "YES" if home > away else "NO"
        if condition == "away_win":
            return "YES" if away > home else "NO"
        if condition == "draw":
            return "YES" if home == away else "NO"
        if condition == "btts":                     # both teams to score
            return "YES" if home > 0 and away > 0 else "NO"
        if condition.startswith("over_"):
            threshold = float(condition.split("_")[1])
            return "YES" if (home + away) > threshold else "NO"
        if condition.startswith("under_"):
            threshold = float(condition.split("_")[1])
            return "YES" if (home + away) < threshold else "NO"

        logger.warning("Unknown condition '%s', returning VOID", condition)
        return "VOID"


# ---------------------------------------------------------------------------
# Weather resolver — OpenWeatherMap
# ---------------------------------------------------------------------------

class WeatherResolver:
    """
    Resolves weather-based prediction markets using OpenWeatherMap.

    Expected market metadata keys:
      city        : str       city name or "lat,lon"
      condition   : str       "above_temp_X" | "below_temp_X" | "rain" | "snow" | "clear"
      unit        : str       "celsius" (default) | "fahrenheit"
      target_date : str       ISO date string (YYYY-MM-DD) — must be within 5-day forecast
    """

    BASE_URL = settings.openweathermap_base_url

    async def resolve(self, ctx: MarketContext) -> ResolutionProposal | None:
        city = ctx.metadata.get("city")
        condition = ctx.metadata.get("condition")
        target_date = ctx.metadata.get("target_date")

        if not all([city, condition, target_date]):
            logger.warning("WeatherResolver: missing metadata for market %s", ctx.market_id)
            return None

        # Use current weather for today, forecast for future dates
        if target_date == datetime.utcnow().date().isoformat():
            endpoint = "weather"
            params: dict = {"q": city, "appid": settings.openweathermap_key, "units": "metric"}
        else:
            endpoint = "forecast"
            params = {"q": city, "appid": settings.openweathermap_key, "units": "metric"}

        async with httpx.AsyncClient(timeout=10) as client:
            resp = await client.get(f"{self.BASE_URL}/{endpoint}", params=params)
            resp.raise_for_status()
            data = resp.json()

        weather_data = self._extract_for_date(data, target_date, endpoint)
        if weather_data is None:
            return None

        outcome = self._evaluate_condition(condition, weather_data)
        evidence = json.dumps({
            "city": city,
            "target_date": target_date,
            "condition": condition,
            "observed": weather_data,
        })

        return ResolutionProposal(
            market_id=ctx.market_id,
            outcome=outcome,
            source="weather_api",
            evidence=evidence,
            confidence=0.90,
        )

    def _extract_for_date(self, data: dict, target_date: str, endpoint: str) -> dict | None:
        if endpoint == "weather":
            return {
                "temp_c": data["main"]["temp"],
                "weather_main": data["weather"][0]["main"].lower(),
            }

        # 3-hour forecast list — find entry closest to noon on target_date
        for entry in data.get("list", []):
            dt_txt: str = entry["dt_txt"]         # "2024-12-25 12:00:00"
            if dt_txt.startswith(target_date) and "12:00:00" in dt_txt:
                return {
                    "temp_c": entry["main"]["temp"],
                    "weather_main": entry["weather"][0]["main"].lower(),
                }
        return None

    def _evaluate_condition(self, condition: str, weather: dict) -> str:
        temp = weather["temp_c"]
        weather_main = weather["weather_main"]

        if condition.startswith("above_temp_"):
            threshold = float(condition.replace("above_temp_", ""))
            return "YES" if temp > threshold else "NO"
        if condition.startswith("below_temp_"):
            threshold = float(condition.replace("below_temp_", ""))
            return "YES" if temp < threshold else "NO"
        if condition == "rain":
            return "YES" if weather_main in ("rain", "drizzle", "thunderstorm") else "NO"
        if condition == "snow":
            return "YES" if weather_main == "snow" else "NO"
        if condition == "clear":
            return "YES" if weather_main == "clear" else "NO"

        logger.warning("Unknown weather condition '%s', returning VOID", condition)
        return "VOID"


# ---------------------------------------------------------------------------
# Resolver registry
# ---------------------------------------------------------------------------

RESOLVERS: dict[str, SportsResolver | WeatherResolver] = {
    "sports": SportsResolver(),
    "weather": WeatherResolver(),
}


async def auto_resolve(ctx: MarketContext) -> ResolutionProposal | None:
    """
    Dispatch to the appropriate resolver based on market category.
    Returns None if the market cannot yet be resolved automatically.
    """
    resolver = RESOLVERS.get(ctx.category)
    if resolver is None:
        logger.debug("No auto-resolver for category '%s'", ctx.category)
        return None

    try:
        return await resolver.resolve(ctx)
    except httpx.HTTPError as exc:
        logger.error("HTTP error in auto_resolve for market %s: %s", ctx.market_id, exc)
        return None
    except Exception as exc:
        logger.exception("Unexpected error in auto_resolve for market %s: %s", ctx.market_id, exc)
        return None
