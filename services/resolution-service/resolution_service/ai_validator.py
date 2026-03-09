"""
AI validation layer using Claude API.

Two responsibilities:
1. validate_proposal()  — cross-check an auto-resolved proposal before confirming.
2. review_appeal()      — reason over an appeal and recommend UPHOLD or REJECT.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import Any
from uuid import UUID

import anthropic

from .config import get_settings
from .resolvers import MarketContext, ResolutionProposal

logger = logging.getLogger(__name__)
settings = get_settings()

_client = anthropic.Anthropic(api_key=settings.anthropic_api_key)


@dataclass
class ValidationResult:
    outcome: str            # "YES" | "NO" | "VOID"
    confidence: float       # 0.0–1.0
    reasoning: str
    needs_human: bool       # True if confidence < threshold


@dataclass
class AppealReviewResult:
    recommendation: str     # "UPHOLD" | "REJECT"
    confidence: float
    reasoning: str
    needs_escalation: bool  # True if confidence < threshold


# ---------------------------------------------------------------------------
# Prompt helpers
# ---------------------------------------------------------------------------

def _validation_prompt(ctx: MarketContext, proposal: ResolutionProposal) -> str:
    return f"""You are a dispute-resolution expert for a prediction market platform.

Market question: {ctx.question}
Resolution criteria: {ctx.resolution_criteria}
Category: {ctx.category}

An automated system has proposed the following resolution:
  Outcome: {proposal.outcome}
  Source: {proposal.source}
  Evidence: {proposal.evidence}

Your task:
1. Verify the evidence is internally consistent with the resolution criteria.
2. Identify any ambiguity, data errors, or edge cases.
3. Produce a final outcome: YES, NO, or VOID (use VOID only if the market is genuinely unresolvable).
4. Give a confidence score between 0.00 and 1.00.

Respond ONLY with valid JSON matching this schema:
{{
  "outcome": "YES" | "NO" | "VOID",
  "confidence": <float 0.00-1.00>,
  "reasoning": "<one-paragraph explanation>"
}}"""


def _appeal_prompt(
    ctx: MarketContext,
    resolution_outcome: str,
    resolution_evidence: str,
    grounds: str,
) -> str:
    return f"""You are an impartial appeals judge for a prediction market platform.

Market question: {ctx.question}
Resolution criteria: {ctx.resolution_criteria}

Original resolution: {resolution_outcome}
Evidence used: {resolution_evidence}

Appellant's grounds for appeal:
{grounds}

Review the appeal carefully.
- If the appellant raises a valid point that the resolution was incorrect, recommend UPHOLD.
- If the original resolution was correct based on the evidence and criteria, recommend REJECT.
- Give a confidence score between 0.00 and 1.00.

Respond ONLY with valid JSON matching this schema:
{{
  "recommendation": "UPHOLD" | "REJECT",
  "confidence": <float 0.00-1.00>,
  "reasoning": "<one-paragraph explanation>"
}}"""


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

async def validate_proposal(
    ctx: MarketContext, proposal: ResolutionProposal
) -> ValidationResult:
    """
    Use Claude to cross-validate an auto-resolved proposal.
    Returns a ValidationResult; sets needs_human=True if confidence is below threshold.
    """
    prompt = _validation_prompt(ctx, proposal)

    try:
        message = _client.messages.create(
            model=settings.anthropic_model,
            max_tokens=512,
            messages=[{"role": "user", "content": prompt}],
        )
        raw = message.content[0].text.strip()
        parsed: dict[str, Any] = json.loads(raw)
    except (anthropic.APIError, json.JSONDecodeError, IndexError) as exc:
        logger.error("AI validation failed for market %s: %s", ctx.market_id, exc)
        # Fall back to original proposal with low confidence → will trigger human review
        return ValidationResult(
            outcome=proposal.outcome,
            confidence=0.0,
            reasoning=f"AI validation error: {exc}",
            needs_human=True,
        )

    confidence = float(parsed.get("confidence", 0.0))
    return ValidationResult(
        outcome=parsed.get("outcome", "VOID"),
        confidence=confidence,
        reasoning=parsed.get("reasoning", ""),
        needs_human=confidence < settings.ai_confidence_threshold,
    )


async def review_appeal(
    ctx: MarketContext,
    resolution_outcome: str,
    resolution_evidence: str,
    grounds: str,
    appeal_id: UUID,
) -> AppealReviewResult:
    """
    Use Claude to review an appeal and recommend UPHOLD or REJECT.
    Escalates to human panel if confidence < threshold.
    """
    prompt = _appeal_prompt(ctx, resolution_outcome, resolution_evidence, grounds)

    try:
        message = _client.messages.create(
            model=settings.anthropic_model,
            max_tokens=512,
            messages=[{"role": "user", "content": prompt}],
        )
        raw = message.content[0].text.strip()
        parsed = json.loads(raw)
    except (anthropic.APIError, json.JSONDecodeError, IndexError) as exc:
        logger.error("AI appeal review failed for appeal %s: %s", appeal_id, exc)
        return AppealReviewResult(
            recommendation="REJECT",
            confidence=0.0,
            reasoning=f"AI review error: {exc}",
            needs_escalation=True,
        )

    confidence = float(parsed.get("confidence", 0.0))
    return AppealReviewResult(
        recommendation=parsed.get("recommendation", "REJECT"),
        confidence=confidence,
        reasoning=parsed.get("reasoning", ""),
        needs_escalation=confidence < settings.ai_confidence_threshold,
    )
