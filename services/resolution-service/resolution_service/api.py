"""
FastAPI router — all HTTP endpoints for the Resolution Service.

Mounted at /v1/resolution in the main app.

Endpoints:
  POST   /resolution/trigger/{market_id}           — manually trigger resolution check
  GET    /resolution/{resolution_id}               — get resolution details
  GET    /resolution/market/{market_id}            — get resolution for a market

  POST   /judges/assign                            — assign judge to a market
  POST   /judges/decide/{assignment_id}            — judge submits decision
  GET    /judges/assignments/{market_id}           — list judge assignments for a market

  POST   /community/report/{market_id}             — submit community dispute report
  GET    /community/reports/{resolution_id}        — list reports for a resolution
  PATCH  /community/reports/{report_id}/reviewed   — mark report as reviewed (admin)

  POST   /appeals/{market_id}                      — submit appeal
  GET    /appeals/{appeal_id}                      — get appeal status
  POST   /appeals/{appeal_id}/panel-decision       — human panel submits decision

  GET    /insurance/fund                           — get insurance fund summary
"""

from __future__ import annotations

from datetime import datetime
from typing import Optional
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from . import appeals as appeals_svc
from . import community as community_svc
from . import insurance as insurance_svc
from . import judges as judges_svc
from .ai_validator import validate_proposal
from .database import get_db
from .models import Resolution, ResolutionStatus
from .resolvers import MarketContext, auto_resolve

router = APIRouter(prefix="/v1/resolution", tags=["resolution"])


# ---------------------------------------------------------------------------
# Pydantic schemas
# ---------------------------------------------------------------------------

class MarketContextPayload(BaseModel):
    question: str
    resolution_criteria: str
    category: str
    metadata: dict = {}
    closes_at: datetime


class TriggerResolutionRequest(BaseModel):
    market_ctx: MarketContextPayload


class AssignJudgeRequest(BaseModel):
    market_id: UUID
    resolution_id: UUID
    judge_user_id: UUID


class JudgeDecisionRequest(BaseModel):
    judge_user_id: UUID
    outcome: str        # "YES" | "NO" | "VOID"
    notes: str


class CommunityReportRequest(BaseModel):
    resolution_id: UUID
    reporter_user_id: UUID
    reason: str
    description: Optional[str] = None


class SubmitAppealRequest(BaseModel):
    resolution_id: UUID
    appellant_user_id: UUID
    grounds: str
    market_ctx: MarketContextPayload


class PanelDecisionRequest(BaseModel):
    panel_member_user_id: UUID
    decision: str       # "UPHOLD" | "REJECT"
    notes: str
    market_ctx: MarketContextPayload


# ---------------------------------------------------------------------------
# Resolution endpoints
# ---------------------------------------------------------------------------

@router.post("/trigger/{market_id}", summary="Manually trigger resolution check")
async def trigger_resolution(
    market_id: UUID,
    body: TriggerResolutionRequest,
    db: AsyncSession = Depends(get_db),
):
    ctx = MarketContext(
        market_id=market_id,
        question=body.market_ctx.question,
        resolution_criteria=body.market_ctx.resolution_criteria,
        category=body.market_ctx.category,
        metadata=body.market_ctx.metadata,
        closes_at=body.market_ctx.closes_at,
    )

    proposal = await auto_resolve(ctx)
    if proposal is None:
        return {"status": "no_data", "message": "Market cannot be resolved yet via automated APIs"}

    validation = await validate_proposal(ctx, proposal)

    from .models import Resolution as ResolutionModel
    from .events import emit_market_resolved
    from datetime import timedelta

    resolution = ResolutionModel(
        market_id=market_id,
        outcome=validation.outcome,
        source=proposal.source,
        status=ResolutionStatus.proposed,
        confidence=validation.confidence,
        evidence=proposal.evidence,
        dispute_window_ends_at=datetime.utcnow() + timedelta(minutes=15),
    )
    db.add(resolution)
    await db.commit()
    await db.refresh(resolution)

    return {
        "resolution_id": resolution.id,
        "outcome": resolution.outcome,
        "confidence": float(resolution.confidence or 0),
        "needs_human": validation.needs_human,
        "status": resolution.status,
        "dispute_window_ends_at": resolution.dispute_window_ends_at,
    }


@router.get("/{resolution_id}", summary="Get resolution details")
async def get_resolution(
    resolution_id: UUID,
    db: AsyncSession = Depends(get_db),
):
    stmt = select(Resolution).where(Resolution.id == resolution_id)
    resolution = (await db.execute(stmt)).scalar_one_or_none()
    if not resolution:
        raise HTTPException(status_code=404, detail="Resolution not found")
    return {
        "id": resolution.id,
        "market_id": resolution.market_id,
        "outcome": resolution.outcome,
        "source": resolution.source,
        "status": resolution.status,
        "confidence": float(resolution.confidence or 0),
        "evidence": resolution.evidence,
        "proposed_at": resolution.proposed_at,
        "confirmed_at": resolution.confirmed_at,
        "dispute_window_ends_at": resolution.dispute_window_ends_at,
    }


@router.get("/market/{market_id}", summary="Get resolution for a market")
async def get_market_resolution(
    market_id: UUID,
    db: AsyncSession = Depends(get_db),
):
    stmt = select(Resolution).where(Resolution.market_id == market_id)
    resolution = (await db.execute(stmt)).scalar_one_or_none()
    if not resolution:
        raise HTTPException(status_code=404, detail="No resolution found for this market")
    return {
        "id": resolution.id,
        "market_id": resolution.market_id,
        "outcome": resolution.outcome,
        "source": resolution.source,
        "status": resolution.status,
        "confidence": float(resolution.confidence or 0),
        "proposed_at": resolution.proposed_at,
        "confirmed_at": resolution.confirmed_at,
    }


# ---------------------------------------------------------------------------
# Judge endpoints
# ---------------------------------------------------------------------------

judges_router = APIRouter(prefix="/v1/judges", tags=["judges"])


@judges_router.post("/assign", summary="Assign judge to a market")
async def assign_judge(
    body: AssignJudgeRequest,
    db: AsyncSession = Depends(get_db),
):
    assignment = await judges_svc.assign_judge(
        db=db,
        market_id=body.market_id,
        resolution_id=body.resolution_id,
        judge_user_id=body.judge_user_id,
    )
    return {
        "assignment_id": assignment.id,
        "market_id": assignment.market_id,
        "judge_user_id": assignment.judge_user_id,
        "status": assignment.status,
        "deadline_at": assignment.deadline_at,
    }


@judges_router.post("/decide/{assignment_id}", summary="Judge submits resolution decision")
async def judge_decide(
    assignment_id: UUID,
    body: JudgeDecisionRequest,
    db: AsyncSession = Depends(get_db),
):
    try:
        assignment = await judges_svc.submit_judge_decision(
            db=db,
            assignment_id=assignment_id,
            judge_user_id=body.judge_user_id,
            outcome=body.outcome,
            notes=body.notes,
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))

    return {
        "assignment_id": assignment.id,
        "outcome": assignment.outcome,
        "status": assignment.status,
        "resolved_at": assignment.resolved_at,
    }


@judges_router.get("/assignments/{market_id}", summary="List judge assignments for a market")
async def list_assignments(
    market_id: UUID,
    db: AsyncSession = Depends(get_db),
):
    from .models import JudgeAssignment
    stmt = select(JudgeAssignment).where(JudgeAssignment.market_id == market_id)
    result = await db.execute(stmt)
    assignments = result.scalars().all()
    return [
        {
            "id": a.id,
            "judge_user_id": a.judge_user_id,
            "status": a.status,
            "outcome": a.outcome,
            "deadline_at": a.deadline_at,
        }
        for a in assignments
    ]


# ---------------------------------------------------------------------------
# Community reporting endpoints
# ---------------------------------------------------------------------------

community_router = APIRouter(prefix="/v1/community", tags=["community"])


@community_router.post("/report/{market_id}", summary="Submit a community dispute report")
async def submit_community_report(
    market_id: UUID,
    body: CommunityReportRequest,
    db: AsyncSession = Depends(get_db),
):
    try:
        report = await community_svc.submit_report(
            db=db,
            market_id=market_id,
            resolution_id=body.resolution_id,
            reporter_user_id=body.reporter_user_id,
            reason=body.reason,
            description=body.description,
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))

    return {
        "report_id": report.id,
        "market_id": report.market_id,
        "reason": report.reason,
        "created_at": report.created_at,
    }


@community_router.get("/reports/{resolution_id}", summary="List community reports for a resolution")
async def list_reports(
    resolution_id: UUID,
    unreviewed_only: bool = False,
    db: AsyncSession = Depends(get_db),
):
    reports = await community_svc.list_reports(db, resolution_id, unreviewed_only)
    return [
        {
            "id": r.id,
            "reporter_user_id": r.reporter_user_id,
            "reason": r.reason,
            "description": r.description,
            "reviewed": r.reviewed,
            "created_at": r.created_at,
        }
        for r in reports
    ]


@community_router.patch("/reports/{report_id}/reviewed", summary="Mark a report as reviewed (admin)")
async def mark_reviewed(
    report_id: UUID,
    db: AsyncSession = Depends(get_db),
):
    try:
        report = await community_svc.mark_report_reviewed(db, report_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))
    return {"report_id": report.id, "reviewed": report.reviewed}


# ---------------------------------------------------------------------------
# Appeals endpoints
# ---------------------------------------------------------------------------

appeals_router = APIRouter(prefix="/v1/appeals", tags=["appeals"])


@appeals_router.post("/{market_id}", summary="Submit an appeal", status_code=status.HTTP_201_CREATED)
async def submit_appeal(
    market_id: UUID,
    body: SubmitAppealRequest,
    db: AsyncSession = Depends(get_db),
):
    ctx = MarketContext(
        market_id=market_id,
        question=body.market_ctx.question,
        resolution_criteria=body.market_ctx.resolution_criteria,
        category=body.market_ctx.category,
        metadata=body.market_ctx.metadata,
        closes_at=body.market_ctx.closes_at,
    )
    try:
        appeal = await appeals_svc.submit_appeal(
            db=db,
            market_id=market_id,
            resolution_id=body.resolution_id,
            appellant_user_id=body.appellant_user_id,
            grounds=body.grounds,
            market_ctx=ctx,
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))

    return {
        "appeal_id": appeal.id,
        "status": appeal.status,
        "ai_recommendation": appeal.ai_recommendation,
        "ai_confidence": float(appeal.ai_confidence or 0),
        "needs_escalation": appeal.status == "escalated",
        "submitted_at": appeal.submitted_at,
    }


@appeals_router.get("/{appeal_id}", summary="Get appeal status")
async def get_appeal(
    appeal_id: UUID,
    db: AsyncSession = Depends(get_db),
):
    try:
        appeal = await appeals_svc.get_appeal(db, appeal_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))

    return {
        "appeal_id": appeal.id,
        "market_id": appeal.market_id,
        "status": appeal.status,
        "grounds": appeal.grounds,
        "ai_recommendation": appeal.ai_recommendation,
        "ai_confidence": float(appeal.ai_confidence or 0),
        "ai_reasoning": appeal.ai_reasoning,
        "panel_decision": appeal.panel_decision,
        "panel_notes": appeal.panel_notes,
        "submitted_at": appeal.submitted_at,
        "decided_at": appeal.decided_at,
    }


@appeals_router.post("/{appeal_id}/panel-decision", summary="Human panel submits final decision")
async def panel_decision(
    appeal_id: UUID,
    body: PanelDecisionRequest,
    db: AsyncSession = Depends(get_db),
):
    ctx = MarketContext(
        market_id=UUID(int=0),  # placeholder — not used in panel path
        question=body.market_ctx.question,
        resolution_criteria=body.market_ctx.resolution_criteria,
        category=body.market_ctx.category,
        metadata=body.market_ctx.metadata,
        closes_at=body.market_ctx.closes_at,
    )
    try:
        appeal = await appeals_svc.submit_panel_decision(
            db=db,
            appeal_id=appeal_id,
            panel_member_user_id=body.panel_member_user_id,
            decision=body.decision,
            notes=body.notes,
            market_ctx=ctx,
        )
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))

    return {
        "appeal_id": appeal.id,
        "status": appeal.status,
        "panel_decision": appeal.panel_decision,
        "decided_at": appeal.decided_at,
    }


# ---------------------------------------------------------------------------
# Insurance fund endpoints
# ---------------------------------------------------------------------------

insurance_router = APIRouter(prefix="/v1/insurance", tags=["insurance"])


@insurance_router.get("/fund", summary="Get insurance fund summary")
async def get_fund(db: AsyncSession = Depends(get_db)):
    summary = await insurance_svc.get_fund_summary(db)
    return {"fund": summary}


@insurance_router.post("/fund/collect", summary="Collect insurance fee from a resolved market (internal)")
async def collect_fee(
    market_id: UUID,
    pool_amount_minor: int,
    currency: str = "USD",
    db: AsyncSession = Depends(get_db),
):
    txn = await insurance_svc.collect_market_fee(db, market_id, pool_amount_minor, currency)
    if txn is None:
        return {"status": "skipped", "reason": "fee is 0"}
    return {
        "txn_id": txn.id,
        "amount_minor": txn.amount_minor,
        "currency": txn.currency,
        "created_at": txn.created_at,
    }
