"""
Community reporting — users flag disputed resolutions.

When a threshold of reports is reached within the dispute window,
the resolution is automatically escalated to "disputed" status,
pausing settlement and opening the appeal window.
"""

from __future__ import annotations

import logging
from uuid import UUID

from sqlalchemy import func, select, update
from sqlalchemy.ext.asyncio import AsyncSession

from .events import emit_resolution_disputed
from .models import CommunityReport, ReportReason, Resolution, ResolutionStatus

logger = logging.getLogger(__name__)

# Minimum number of community reports to trigger dispute escalation
REPORT_THRESHOLD = 5


async def submit_report(
    db: AsyncSession,
    market_id: UUID,
    resolution_id: UUID,
    reporter_user_id: UUID,
    reason: str,
    description: str | None = None,
) -> CommunityReport:
    """
    Record a community dispute report.
    If REPORT_THRESHOLD is reached, escalate the resolution to 'disputed'.
    """
    # Prevent duplicate reports from the same user
    existing_stmt = select(CommunityReport).where(
        CommunityReport.resolution_id == resolution_id,
        CommunityReport.reporter_user_id == reporter_user_id,
    )
    existing = (await db.execute(existing_stmt)).scalar_one_or_none()
    if existing:
        raise ValueError("User has already reported this resolution")

    # Verify resolution is in a reportable state
    res_stmt = select(Resolution).where(Resolution.id == resolution_id)
    resolution = (await db.execute(res_stmt)).scalar_one_or_none()
    if resolution is None:
        raise ValueError(f"Resolution {resolution_id} not found")
    if resolution.status not in (ResolutionStatus.proposed, ResolutionStatus.confirmed):
        raise ValueError(f"Resolution {resolution_id} is not in a reportable state")

    report = CommunityReport(
        resolution_id=resolution_id,
        market_id=market_id,
        reporter_user_id=reporter_user_id,
        reason=ReportReason(reason),
        description=description,
    )
    db.add(report)
    await db.flush()

    # Count total reports for this resolution
    count_stmt = select(func.count()).where(
        CommunityReport.resolution_id == resolution_id
    )
    report_count: int = (await db.execute(count_stmt)).scalar_one()

    if report_count >= REPORT_THRESHOLD:
        await _escalate_to_disputed(db, resolution_id, market_id, reporter_user_id)

    await db.commit()
    await db.refresh(report)
    logger.info(
        "Community report submitted for resolution %s by user %s (%d/%d)",
        resolution_id, reporter_user_id, report_count, REPORT_THRESHOLD,
    )
    return report


async def _escalate_to_disputed(
    db: AsyncSession,
    resolution_id: UUID,
    market_id: UUID,
    reporter_user_id: UUID,
) -> None:
    """Mark resolution as disputed, halting settlement."""
    await db.execute(
        update(Resolution)
        .where(Resolution.id == resolution_id)
        .values(status=ResolutionStatus.disputed)
    )
    await emit_resolution_disputed(market_id, resolution_id, reporter_user_id)
    logger.warning(
        "Resolution %s for market %s escalated to DISPUTED after community reports",
        resolution_id, market_id,
    )


async def list_reports(
    db: AsyncSession,
    resolution_id: UUID,
    unreviewed_only: bool = False,
) -> list[CommunityReport]:
    stmt = select(CommunityReport).where(CommunityReport.resolution_id == resolution_id)
    if unreviewed_only:
        stmt = stmt.where(CommunityReport.reviewed.is_(False))
    result = await db.execute(stmt)
    return list(result.scalars().all())


async def mark_report_reviewed(db: AsyncSession, report_id: UUID) -> CommunityReport:
    stmt = select(CommunityReport).where(CommunityReport.id == report_id)
    report = (await db.execute(stmt)).scalar_one_or_none()
    if report is None:
        raise ValueError(f"Report {report_id} not found")
    report.reviewed = True
    await db.commit()
    await db.refresh(report)
    return report
