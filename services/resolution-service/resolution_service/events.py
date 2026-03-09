"""
Kafka event publisher for the Resolution Service.

All significant events are published so downstream services
(Settlement, Notifications, Analytics) can react asynchronously.
"""

from __future__ import annotations

import json
import logging
from datetime import datetime
from typing import Any
from uuid import UUID

from aiokafka import AIOKafkaProducer

from .config import get_settings

logger = logging.getLogger(__name__)
settings = get_settings()

_producer: AIOKafkaProducer | None = None


async def get_producer() -> AIOKafkaProducer:
    global _producer
    if _producer is None:
        _producer = AIOKafkaProducer(
            bootstrap_servers=settings.kafka_bootstrap_servers,
            value_serializer=lambda v: json.dumps(v, default=str).encode(),
        )
        await _producer.start()
    return _producer


async def stop_producer() -> None:
    global _producer
    if _producer:
        await _producer.stop()
        _producer = None


async def _publish(topic: str, payload: dict[str, Any]) -> None:
    try:
        producer = await get_producer()
        await producer.send_and_wait(topic, payload)
        logger.debug("Published to %s: %s", topic, payload)
    except Exception as exc:
        logger.error("Failed to publish to %s: %s", topic, exc)


# ---------------------------------------------------------------------------
# Event helpers
# ---------------------------------------------------------------------------

async def emit_market_resolved(
    market_id: UUID,
    resolution_id: UUID,
    outcome: str,
    source: str,
) -> None:
    await _publish("markets.resolved", {
        "event": "market_resolved",
        "market_id": str(market_id),
        "resolution_id": str(resolution_id),
        "outcome": outcome,
        "source": source,
        "timestamp": datetime.utcnow().isoformat(),
    })


async def emit_resolution_disputed(
    market_id: UUID,
    resolution_id: UUID,
    reporter_user_id: UUID,
) -> None:
    await _publish("markets.disputed", {
        "event": "resolution_disputed",
        "market_id": str(market_id),
        "resolution_id": str(resolution_id),
        "reporter_user_id": str(reporter_user_id),
        "timestamp": datetime.utcnow().isoformat(),
    })


async def emit_appeal_decided(
    market_id: UUID,
    appeal_id: UUID,
    decision: str,     # "UPHOLD" | "REJECT"
) -> None:
    await _publish("markets.appeal_decided", {
        "event": "appeal_decided",
        "market_id": str(market_id),
        "appeal_id": str(appeal_id),
        "decision": decision,
        "timestamp": datetime.utcnow().isoformat(),
    })


async def emit_judge_assigned(market_id: UUID, judge_user_id: UUID) -> None:
    await _publish("markets.judge_assigned", {
        "event": "judge_assigned",
        "market_id": str(market_id),
        "judge_user_id": str(judge_user_id),
        "timestamp": datetime.utcnow().isoformat(),
    })
