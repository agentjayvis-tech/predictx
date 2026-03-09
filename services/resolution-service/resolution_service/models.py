"""
SQLAlchemy ORM models for the Resolution Service.

Tables owned by this service:
  - resolutions          : final outcomes for markets
  - judge_assignments    : judge → market assignments
  - community_reports    : user dispute flags
  - appeals              : formal appeals of resolutions
  - insurance_fund_txns  : insurance pool ledger
"""

import enum
import uuid
from datetime import datetime

from sqlalchemy import (
    BigInteger, Boolean, Column, DateTime, Enum, ForeignKey,
    Integer, Numeric, String, Text, func,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import DeclarativeBase, relationship


class Base(DeclarativeBase):
    pass


# ---------------------------------------------------------------------------
# Enums
# ---------------------------------------------------------------------------

class ResolutionSource(str, enum.Enum):
    sports_api = "sports_api"
    weather_api = "weather_api"
    ai_validated = "ai_validated"
    judge = "judge"
    community = "community"


class ResolutionOutcome(str, enum.Enum):
    yes = "YES"
    no = "NO"
    void = "VOID"   # cancelled / insufficient data


class ResolutionStatus(str, enum.Enum):
    proposed = "proposed"       # auto-resolution proposed, in dispute window
    confirmed = "confirmed"     # dispute window passed, sent to settlement
    disputed = "disputed"       # flagged by community / appeal open
    overturned = "overturned"   # appeal upheld, original resolution reversed
    void = "void"


class JudgeAssignmentStatus(str, enum.Enum):
    pending = "pending"
    accepted = "accepted"
    resolved = "resolved"
    expired = "expired"


class ReportReason(str, enum.Enum):
    wrong_outcome = "wrong_outcome"
    data_error = "data_error"
    market_ambiguous = "market_ambiguous"
    other = "other"


class AppealStatus(str, enum.Enum):
    submitted = "submitted"
    ai_review = "ai_review"
    escalated = "escalated"     # confidence < threshold → human panel
    upheld = "upheld"           # appeal granted
    rejected = "rejected"       # appeal denied


class InsuranceTxnType(str, enum.Enum):
    deposit = "deposit"         # fee collected from market pool
    payout = "payout"           # paid out to users in dispute
    refund = "refund"           # returned to market if not used


# ---------------------------------------------------------------------------
# Models
# ---------------------------------------------------------------------------

class Resolution(Base):
    __tablename__ = "resolutions"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    market_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    outcome = Column(Enum(ResolutionOutcome), nullable=False)
    source = Column(Enum(ResolutionSource), nullable=False)
    status = Column(Enum(ResolutionStatus), nullable=False, default=ResolutionStatus.proposed)

    # Confidence score from AI validator (0.0–1.0)
    confidence = Column(Numeric(4, 3), nullable=True)

    # Raw evidence payload (API response JSON / judge notes / AI reasoning)
    evidence = Column(Text, nullable=True)

    # Timestamps
    proposed_at = Column(DateTime(timezone=True), server_default=func.now())
    confirmed_at = Column(DateTime(timezone=True), nullable=True)
    dispute_window_ends_at = Column(DateTime(timezone=True), nullable=True)

    judge_assignments = relationship("JudgeAssignment", back_populates="resolution")
    community_reports = relationship("CommunityReport", back_populates="resolution")
    appeals = relationship("Appeal", back_populates="resolution")


class JudgeAssignment(Base):
    __tablename__ = "judge_assignments"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    resolution_id = Column(UUID(as_uuid=True), ForeignKey("resolutions.id"), nullable=False)
    market_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    judge_user_id = Column(UUID(as_uuid=True), nullable=False)

    status = Column(Enum(JudgeAssignmentStatus), nullable=False, default=JudgeAssignmentStatus.pending)
    outcome = Column(Enum(ResolutionOutcome), nullable=True)
    notes = Column(Text, nullable=True)

    assigned_at = Column(DateTime(timezone=True), server_default=func.now())
    deadline_at = Column(DateTime(timezone=True), nullable=False)
    resolved_at = Column(DateTime(timezone=True), nullable=True)

    resolution = relationship("Resolution", back_populates="judge_assignments")


class CommunityReport(Base):
    __tablename__ = "community_reports"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    resolution_id = Column(UUID(as_uuid=True), ForeignKey("resolutions.id"), nullable=False)
    market_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    reporter_user_id = Column(UUID(as_uuid=True), nullable=False)

    reason = Column(Enum(ReportReason), nullable=False)
    description = Column(Text, nullable=True)
    reviewed = Column(Boolean, default=False)

    created_at = Column(DateTime(timezone=True), server_default=func.now())

    resolution = relationship("Resolution", back_populates="community_reports")


class Appeal(Base):
    __tablename__ = "appeals"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    resolution_id = Column(UUID(as_uuid=True), ForeignKey("resolutions.id"), nullable=False)
    market_id = Column(UUID(as_uuid=True), nullable=False, index=True)
    appellant_user_id = Column(UUID(as_uuid=True), nullable=False)

    status = Column(Enum(AppealStatus), nullable=False, default=AppealStatus.submitted)
    grounds = Column(Text, nullable=False)

    # AI review
    ai_recommendation = Column(String(10), nullable=True)   # "UPHOLD" | "REJECT"
    ai_confidence = Column(Numeric(4, 3), nullable=True)
    ai_reasoning = Column(Text, nullable=True)

    # Human panel (populated if escalated)
    panel_decision = Column(String(10), nullable=True)
    panel_notes = Column(Text, nullable=True)
    panel_decided_by = Column(UUID(as_uuid=True), nullable=True)

    submitted_at = Column(DateTime(timezone=True), server_default=func.now())
    decided_at = Column(DateTime(timezone=True), nullable=True)

    resolution = relationship("Resolution", back_populates="appeals")


class InsuranceFundTxn(Base):
    __tablename__ = "insurance_fund_txns"

    id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    market_id = Column(UUID(as_uuid=True), nullable=True, index=True)
    appeal_id = Column(UUID(as_uuid=True), ForeignKey("appeals.id"), nullable=True)

    txn_type = Column(Enum(InsuranceTxnType), nullable=False)
    amount_minor = Column(BigInteger, nullable=False)    # smallest currency unit
    currency = Column(String(3), nullable=False, default="USD")
    description = Column(Text, nullable=True)

    created_at = Column(DateTime(timezone=True), server_default=func.now())
