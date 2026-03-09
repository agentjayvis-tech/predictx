"""
Celery Beat tasks for the Resolution Service.

Scheduled tasks:
  - check_markets_for_resolution : every 60 seconds
  - expire_judge_assignments     : every 10 minutes
  - confirm_proposed_resolutions : every 60 seconds (dispute window check)
"""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timedelta
from uuid import UUID

from celery import Celery
from sqlalchemy import select, update

from .config import get_settings
from .database import AsyncSessionLocal
from .judges import expire_overdue_assignments
from .models import Resolution, ResolutionStatus
from .resolvers import MarketContext, auto_resolve
from .ai_validator import validate_proposal

logger = logging.getLogger(__name__)
settings = get_settings()

celery_app = Celery(
    "resolution_service",
    broker=settings.celery_broker_url,
    backend=settings.celery_result_backend,
)

celery_app.conf.beat_schedule = {
    "check-markets-every-60s": {
        "task": "resolution_service.tasks.check_markets_for_resolution",
        "schedule": settings.resolution_check_interval_seconds,
    },
    "expire-judge-assignments-every-10m": {
        "task": "resolution_service.tasks.expire_judge_assignments_task",
        "schedule": 600,
    },
    "confirm-proposed-resolutions-every-60s": {
        "task": "resolution_service.tasks.confirm_proposed_resolutions",
        "schedule": 60,
    },
    "track-user-losses-every-60s": {
        "task": "resolution_service.tasks.track_user_losses",
        "schedule": 60,
    },
}
celery_app.conf.timezone = "UTC"


def _run_async(coro):
    """Run an async coroutine from a synchronous Celery task."""
    loop = asyncio.new_event_loop()
    try:
        return loop.run_until_complete(coro)
    finally:
        loop.close()


# ---------------------------------------------------------------------------
# Task: Check active markets for auto-resolution triggers
# ---------------------------------------------------------------------------

@celery_app.task(name="resolution_service.tasks.check_markets_for_resolution", bind=True, max_retries=3)
def check_markets_for_resolution(self):
    """
    Fetches all active markets from the Market Service (via internal API or shared DB),
    and attempts auto-resolution for each using the appropriate resolver.

    In production this would query the Market Service's read replica or a shared
    events table.  Here we stub the market fetch and process any pending resolutions.
    """
    _run_async(_async_check_markets())


async def _async_check_markets():
    from .events import emit_market_resolved

    # Fetch active markets from Market Service
    # In production: call internal gRPC / REST endpoint or consume Kafka snapshot
    active_markets = await _fetch_active_markets()

    async with AsyncSessionLocal() as db:
        for market_data in active_markets:
            ctx = MarketContext(
                market_id=UUID(market_data["market_id"]),
                question=market_data["question"],
                resolution_criteria=market_data["resolution_criteria"],
                category=market_data["category"],
                metadata=market_data.get("metadata", {}),
                closes_at=datetime.fromisoformat(market_data["closes_at"]),
            )

            # Skip if already resolved
            existing_stmt = select(Resolution).where(
                Resolution.market_id == ctx.market_id,
                Resolution.status.notin_([ResolutionStatus.overturned]),
            )
            if (await db.execute(existing_stmt)).scalar_one_or_none():
                continue

            proposal = await auto_resolve(ctx)
            if proposal is None:
                continue

            # AI cross-validation
            validation = await validate_proposal(ctx, proposal)

            resolution = Resolution(
                market_id=ctx.market_id,
                outcome=validation.outcome,
                source=proposal.source,
                status=ResolutionStatus.proposed,
                confidence=validation.confidence,
                evidence=proposal.evidence,
                dispute_window_ends_at=datetime.utcnow() + timedelta(
                    minutes=settings.dispute_window_minutes
                ),
            )
            db.add(resolution)
            await db.commit()
            await db.refresh(resolution)

            if validation.needs_human:
                logger.info(
                    "Market %s needs human judge (AI confidence=%.2f)",
                    ctx.market_id, validation.confidence,
                )
                # In production: publish event so admin UI can assign a judge
            else:
                logger.info(
                    "Market %s auto-resolved: %s (confidence=%.2f)",
                    ctx.market_id, validation.outcome, validation.confidence,
                )


async def _fetch_active_markets() -> list[dict]:
    """
    Stub: returns active markets from Market Service.
    Replace with actual gRPC / HTTP call in production.
    """
    import httpx
    try:
        async with httpx.AsyncClient(timeout=5) as client:
            resp = await client.get("http://market-service/internal/markets/resolvable")
            resp.raise_for_status()
            return resp.json()
    except Exception as exc:
        logger.error("Failed to fetch active markets: %s", exc)
        return []


# ---------------------------------------------------------------------------
# Task: Expire overdue judge assignments
# ---------------------------------------------------------------------------

@celery_app.task(name="resolution_service.tasks.expire_judge_assignments_task")
def expire_judge_assignments_task():
    _run_async(_async_expire_judges())


async def _async_expire_judges():
    async with AsyncSessionLocal() as db:
        count = await expire_overdue_assignments(db)
        if count:
            logger.info("Expired %d judge assignments", count)


# ---------------------------------------------------------------------------
# Task: Confirm proposed resolutions after dispute window
# ---------------------------------------------------------------------------

@celery_app.task(name="resolution_service.tasks.confirm_proposed_resolutions")
def confirm_proposed_resolutions():
    _run_async(_async_confirm_proposals())


async def _async_confirm_proposals():
    from .events import emit_market_resolved

    now = datetime.utcnow()
    async with AsyncSessionLocal() as db:
        stmt = select(Resolution).where(
            Resolution.status == ResolutionStatus.proposed,
            Resolution.dispute_window_ends_at <= now,
        )
        result = await db.execute(stmt)
        resolutions = result.scalars().all()

        for resolution in resolutions:
            await db.execute(
                update(Resolution)
                .where(Resolution.id == resolution.id)
                .values(
                    status=ResolutionStatus.confirmed,
                    confirmed_at=now,
                )
            )
            await db.commit()
            await emit_market_resolved(
                resolution.market_id,
                resolution.id,
                resolution.outcome.value,
                resolution.source.value,
            )
            logger.info(
                "Resolution %s confirmed for market %s after dispute window",
                resolution.id, resolution.market_id,
            )


# ---------------------------------------------------------------------------
# Task: Track user losses for loss streak notifications
# ---------------------------------------------------------------------------

@celery_app.task(name="resolution_service.tasks.track_user_losses")
def track_user_losses():
    """
    Consumes settlement.completed events from Kafka to track consecutive losses.
    When a user reaches their loss streak threshold, emits a user.loss_streak_alert event.
    """
    _run_async(_async_track_losses())


async def _async_track_losses():
    """
    Consume settlement.completed events and track loss streaks.
    This is a simplified implementation that can be enhanced with Kafka consumer integration.
    """
    from kafka import KafkaConsumer
    import json
    from datetime import datetime

    try:
        # Create consumer for settlement.completed events
        consumer = KafkaConsumer(
            "settlement.completed",
            bootstrap_servers=settings.kafka_bootstrap_servers.split(","),
            group_id="loss-tracking-consumer",
            value_deserializer=lambda m: json.loads(m.decode("utf-8")),
            auto_offset_reset="latest",
            enable_auto_commit=True,
            max_poll_records=100,
        )

        # Process batches of events
        for message in consumer:
            settlement = message.value
            await _process_settlement_for_loss_tracking(settlement)

    except Exception as exc:
        logger.error("Loss tracking consumer error: %s", exc)


async def _process_settlement_for_loss_tracking(settlement: dict) -> None:
    """
    Process a single settlement event to track user losses.

    settlement payload should contain:
    - user_id: UUID string
    - position_payout_minor: int
    - position_stake_minor: int
    - market_id: UUID string
    """
    from .events import _publish
    from sqlalchemy import select, update

    try:
        user_id = UUID(settlement.get("user_id", ""))
        payout = settlement.get("position_payout_minor", 0)
        stake = settlement.get("position_stake_minor", 0)
        market_id = UUID(settlement.get("market_id", ""))

        async with AsyncSessionLocal() as db:
            # Track the loss/win in database
            # This would require creating a user_loss_tracking table in resolution service
            # or calling the wallet service to update it

            is_loss = payout < stake

            if is_loss:
                # Increment consecutive losses
                # In a real implementation, this would query user_loss_tracking
                logger.debug(
                    "User %s lost on market %s: stake=%d, payout=%d",
                    user_id, market_id, stake, payout,
                )

                # Check if user has reached their threshold
                # This would require fetching user's loss_streak_notification_threshold
                # from wallet-service and comparing to consecutive_losses count

                # For now, we publish a generic loss alert
                # In production, check threshold and only publish when exceeded
                await _publish(
                    "user.loss_streak_alert",
                    {
                        "user_id": str(user_id),
                        "market_id": str(market_id),
                        "is_loss": True,
                        "stake_minor": stake,
                        "payout_minor": payout,
                        "timestamp": datetime.utcnow().isoformat(),
                    },
                )
            else:
                # Reset consecutive losses on win
                logger.debug(
                    "User %s won on market %s: stake=%d, payout=%d",
                    user_id, market_id, stake, payout,
                )

    except Exception as exc:
        logger.error("Error processing settlement for loss tracking: %s", exc)
