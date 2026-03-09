"""
Manual judge resolution flow.

When a market cannot be auto-resolved (subjective categories, AI confidence too low,
or auto-resolver returns None), an admin assigns one or more judges.
Judges submit their outcome; majority vote wins.
"""

from __future__ import annotations

import logging
from datetime import datetime, timedelta
from uuid import UUID

from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession

from .config import get_settings
from .events import emit_judge_assigned, emit_market_resolved
from .models import (
    JudgeAssignment, JudgeAssignmentStatus, Resolution,
    ResolutionOutcome, ResolutionSource, ResolutionStatus,
)

logger = logging.getLogger(__name__)
settings = get_settings()


async def assign_judge(
    db: AsyncSession,
    market_id: UUID,
    resolution_id: UUID,
    judge_user_id: UUID,
) -> JudgeAssignment:
    """
    Assign a judge to a market that requires manual resolution.
    Multiple judges can be assigned; majority vote determines outcome.
    """
    deadline = datetime.utcnow() + timedelta(hours=settings.judge_decision_deadline_hours)
    assignment = JudgeAssignment(
        resolution_id=resolution_id,
        market_id=market_id,
        judge_user_id=judge_user_id,
        deadline_at=deadline,
    )
    db.add(assignment)
    await db.commit()
    await db.refresh(assignment)

    await emit_judge_assigned(market_id, judge_user_id)
    logger.info("Judge %s assigned to market %s (deadline: %s)", judge_user_id, market_id, deadline)
    return assignment


async def submit_judge_decision(
    db: AsyncSession,
    assignment_id: UUID,
    judge_user_id: UUID,
    outcome: str,
    notes: str,
) -> JudgeAssignment:
    """
    Record a judge's decision.  After each submission, check if a majority has been reached.
    """
    stmt = select(JudgeAssignment).where(
        JudgeAssignment.id == assignment_id,
        JudgeAssignment.judge_user_id == judge_user_id,
        JudgeAssignment.status == JudgeAssignmentStatus.pending,
    )
    result = await db.execute(stmt)
    assignment = result.scalar_one_or_none()

    if assignment is None:
        raise ValueError(f"Assignment {assignment_id} not found or already resolved")

    if datetime.utcnow() > assignment.deadline_at:
        assignment.status = JudgeAssignmentStatus.expired
        await db.commit()
        raise ValueError(f"Assignment {assignment_id} has expired")

    assignment.outcome = ResolutionOutcome(outcome)
    assignment.notes = notes
    assignment.status = JudgeAssignmentStatus.resolved
    assignment.resolved_at = datetime.utcnow()
    await db.commit()

    # Check for majority among all assignments for this resolution
    await _check_majority(db, assignment.resolution_id, assignment.market_id)

    await db.refresh(assignment)
    return assignment


async def _check_majority(
    db: AsyncSession, resolution_id: UUID, market_id: UUID
) -> None:
    """Resolve the market if all active judges have voted and a majority exists."""
    stmt = select(JudgeAssignment).where(
        JudgeAssignment.resolution_id == resolution_id,
        JudgeAssignment.status.in_(
            [JudgeAssignmentStatus.resolved, JudgeAssignmentStatus.pending]
        ),
    )
    result = await db.execute(stmt)
    assignments = result.scalars().all()

    resolved = [a for a in assignments if a.status == JudgeAssignmentStatus.resolved]
    pending = [a for a in assignments if a.status == JudgeAssignmentStatus.pending]

    # Wait until all judges have voted (or deadline has passed for pending ones)
    now = datetime.utcnow()
    active_pending = [a for a in pending if a.deadline_at > now]
    if active_pending:
        return  # still waiting on judges

    if not resolved:
        return

    # Tally votes
    tally: dict[str, int] = {}
    for a in resolved:
        key = a.outcome.value if a.outcome else "VOID"
        tally[key] = tally.get(key, 0) + 1

    majority_outcome = max(tally, key=lambda k: tally[k])
    total = len(resolved)
    majority_count = tally[majority_outcome]

    # Require strict majority
    if majority_count <= total / 2:
        logger.warning("No majority reached for resolution %s; marking VOID", resolution_id)
        majority_outcome = "VOID"

    # Update resolution
    await db.execute(
        update(Resolution)
        .where(Resolution.id == resolution_id)
        .values(
            outcome=ResolutionOutcome(majority_outcome),
            source=ResolutionSource.judge,
            status=ResolutionStatus.confirmed,
            confirmed_at=datetime.utcnow(),
        )
    )
    await db.commit()

    await emit_market_resolved(market_id, resolution_id, majority_outcome, "judge")
    logger.info(
        "Market %s resolved by judges: %s (votes: %s)", market_id, majority_outcome, tally
    )


async def expire_overdue_assignments(db: AsyncSession) -> int:
    """
    Mark pending assignments past their deadline as expired.
    Called periodically by the Celery cron task.
    Returns count of expired assignments.
    """
    stmt = (
        update(JudgeAssignment)
        .where(
            JudgeAssignment.status == JudgeAssignmentStatus.pending,
            JudgeAssignment.deadline_at < datetime.utcnow(),
        )
        .values(status=JudgeAssignmentStatus.expired)
        .returning(JudgeAssignment.id)
    )
    result = await db.execute(stmt)
    await db.commit()
    expired = result.fetchall()
    if expired:
        logger.info("Expired %d overdue judge assignments", len(expired))
    return len(expired)
