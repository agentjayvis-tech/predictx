"""
Appeal mechanism.

Flow:
  1. User submits appeal with grounds text.
  2. AI reviews evidence vs. grounds → recommends UPHOLD or REJECT.
  3. If AI confidence ≥ threshold → auto-decide.
  4. If AI confidence < threshold → escalate to human panel.
  5. Human panel member submits final decision.
  6. On UPHOLD: resolution is overturned, settlement re-triggered with opposite outcome.
  7. Insurance fund may be tapped for user compensation on upheld appeals.
"""

from __future__ import annotations

import json
import logging
from datetime import datetime
from uuid import UUID

from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession

from .ai_validator import review_appeal
from .config import get_settings
from .events import emit_appeal_decided, emit_market_resolved
from .insurance import credit_insurance_payout
from .models import (
    Appeal, AppealStatus, Resolution, ResolutionOutcome,
    ResolutionSource, ResolutionStatus,
)
from .resolvers import MarketContext

logger = logging.getLogger(__name__)
settings = get_settings()


async def submit_appeal(
    db: AsyncSession,
    market_id: UUID,
    resolution_id: UUID,
    appellant_user_id: UUID,
    grounds: str,
    market_ctx: MarketContext,
) -> Appeal:
    """
    Submit a formal appeal of a resolution.
    Immediately triggers AI review in the background.
    """
    resolution = await _get_resolution(db, resolution_id)

    if resolution.status not in (ResolutionStatus.proposed, ResolutionStatus.confirmed, ResolutionStatus.disputed):
        raise ValueError(f"Resolution {resolution_id} cannot be appealed (status={resolution.status})")

    # One appeal per user per resolution
    existing_stmt = select(Appeal).where(
        Appeal.resolution_id == resolution_id,
        Appeal.appellant_user_id == appellant_user_id,
        Appeal.status.notin_([AppealStatus.rejected]),
    )
    if (await db.execute(existing_stmt)).scalar_one_or_none():
        raise ValueError("An active appeal already exists for this resolution from this user")

    appeal = Appeal(
        resolution_id=resolution_id,
        market_id=market_id,
        appellant_user_id=appellant_user_id,
        grounds=grounds,
        status=AppealStatus.ai_review,
    )
    db.add(appeal)
    await db.flush()
    await db.refresh(appeal)

    # Run AI review synchronously (in production this could be a Celery task)
    ai_result = await review_appeal(
        ctx=market_ctx,
        resolution_outcome=resolution.outcome.value,
        resolution_evidence=resolution.evidence or "",
        grounds=grounds,
        appeal_id=appeal.id,
    )

    appeal.ai_recommendation = ai_result.recommendation
    appeal.ai_confidence = ai_result.confidence
    appeal.ai_reasoning = ai_result.reasoning

    if ai_result.needs_escalation:
        appeal.status = AppealStatus.escalated
        logger.info(
            "Appeal %s escalated to human panel (AI confidence=%.2f)",
            appeal.id, ai_result.confidence,
        )
    else:
        # Auto-decide based on AI recommendation
        await _apply_appeal_decision(db, appeal, market_ctx, ai_result.recommendation)

    await db.commit()
    await db.refresh(appeal)
    return appeal


async def submit_panel_decision(
    db: AsyncSession,
    appeal_id: UUID,
    panel_member_user_id: UUID,
    decision: str,          # "UPHOLD" | "REJECT"
    notes: str,
    market_ctx: MarketContext,
) -> Appeal:
    """Human panel member submits a final decision on an escalated appeal."""
    stmt = select(Appeal).where(
        Appeal.id == appeal_id,
        Appeal.status == AppealStatus.escalated,
    )
    appeal = (await db.execute(stmt)).scalar_one_or_none()
    if appeal is None:
        raise ValueError(f"Appeal {appeal_id} not found or not awaiting panel decision")

    appeal.panel_decision = decision
    appeal.panel_notes = notes
    appeal.panel_decided_by = panel_member_user_id

    await _apply_appeal_decision(db, appeal, market_ctx, decision)
    await db.commit()
    await db.refresh(appeal)
    return appeal


async def _apply_appeal_decision(
    db: AsyncSession,
    appeal: Appeal,
    market_ctx: MarketContext,
    decision: str,
) -> None:
    """Apply the final appeal decision, overturning the resolution if upheld."""
    resolution = await _get_resolution(db, appeal.resolution_id)
    now = datetime.utcnow()

    if decision == "UPHOLD":
        appeal.status = AppealStatus.upheld
        appeal.decided_at = now

        # Flip the outcome
        flipped = _flip_outcome(resolution.outcome)
        await db.execute(
            update(Resolution)
            .where(Resolution.id == resolution.id)
            .values(
                outcome=flipped,
                source=ResolutionSource.ai_validated,
                status=ResolutionStatus.overturned,
            )
        )

        # Tap insurance fund to compensate users who lost due to wrong original resolution
        # Amount is determined by the market pool size; here we signal the event
        await credit_insurance_payout(
            db=db,
            market_id=appeal.market_id,
            appeal_id=appeal.id,
            amount_minor=0,     # Settlement service calculates exact amount from positions
            currency="USD",
            description=f"Appeal {appeal.id} upheld — compensation payout",
        )

        await emit_market_resolved(
            appeal.market_id, resolution.id, flipped.value, "appeal_upheld"
        )
        logger.info(
            "Appeal %s UPHELD — market %s resolution overturned to %s",
            appeal.id, appeal.market_id, flipped.value,
        )
    else:
        appeal.status = AppealStatus.rejected
        appeal.decided_at = now
        logger.info("Appeal %s REJECTED", appeal.id)

    await emit_appeal_decided(appeal.market_id, appeal.id, decision)


def _flip_outcome(outcome: ResolutionOutcome) -> ResolutionOutcome:
    if outcome == ResolutionOutcome.yes:
        return ResolutionOutcome.no
    if outcome == ResolutionOutcome.no:
        return ResolutionOutcome.yes
    return ResolutionOutcome.void


async def _get_resolution(db: AsyncSession, resolution_id: UUID) -> Resolution:
    stmt = select(Resolution).where(Resolution.id == resolution_id)
    res = (await db.execute(stmt)).scalar_one_or_none()
    if res is None:
        raise ValueError(f"Resolution {resolution_id} not found")
    return res


async def get_appeal(db: AsyncSession, appeal_id: UUID) -> Appeal:
    stmt = select(Appeal).where(Appeal.id == appeal_id)
    appeal = (await db.execute(stmt)).scalar_one_or_none()
    if appeal is None:
        raise ValueError(f"Appeal {appeal_id} not found")
    return appeal
