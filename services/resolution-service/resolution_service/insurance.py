"""
Insurance fund for disputes.

A small fee (configurable in basis points) is collected from each market's
liquidity pool when the market resolves.  The accumulated fund is used to
compensate users when an appeal is upheld (i.e., the original resolution
was wrong and settlement already occurred).

Ledger entries use double-entry semantics:
  deposit  — fee collected from market pool (increases fund)
  payout   — compensation sent to users (decreases fund)
  refund   — unused deposit returned if market voided (decreases fund)
"""

from __future__ import annotations

import logging
from decimal import Decimal
from uuid import UUID

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from .config import get_settings
from .models import InsuranceFundTxn, InsuranceTxnType

logger = logging.getLogger(__name__)
settings = get_settings()


async def collect_market_fee(
    db: AsyncSession,
    market_id: UUID,
    pool_amount_minor: int,
    currency: str = "USD",
) -> InsuranceFundTxn:
    """
    Deduct insurance fee from a market's pool on resolution.
    fee = pool_amount_minor * insurance_fund_fee_bps / 10_000
    """
    fee = int(pool_amount_minor * settings.insurance_fund_fee_bps / 10_000)
    if fee <= 0:
        logger.debug("Insurance fee for market %s is 0; skipping", market_id)
        return None  # type: ignore[return-value]

    txn = InsuranceFundTxn(
        market_id=market_id,
        txn_type=InsuranceTxnType.deposit,
        amount_minor=fee,
        currency=currency,
        description=f"Insurance fee ({settings.insurance_fund_fee_bps}bps) from market {market_id}",
    )
    db.add(txn)
    await db.commit()
    await db.refresh(txn)
    logger.info("Insurance fee collected: %d %s from market %s", fee, currency, market_id)
    return txn


async def credit_insurance_payout(
    db: AsyncSession,
    market_id: UUID,
    appeal_id: UUID,
    amount_minor: int,
    currency: str = "USD",
    description: str = "",
) -> InsuranceFundTxn:
    """
    Record a payout from the insurance fund.
    The Settlement Service will process the actual wallet credits;
    this records the authorisation in the fund ledger.
    """
    current_balance = await get_fund_balance(db, currency)
    if amount_minor > 0 and current_balance < amount_minor:
        logger.warning(
            "Insurance fund balance %d %s insufficient for payout of %d; partial cover",
            current_balance, currency, amount_minor,
        )

    txn = InsuranceFundTxn(
        market_id=market_id,
        appeal_id=appeal_id,
        txn_type=InsuranceTxnType.payout,
        amount_minor=amount_minor,
        currency=currency,
        description=description or f"Dispute payout for appeal {appeal_id}",
    )
    db.add(txn)
    await db.commit()
    await db.refresh(txn)
    logger.info("Insurance payout: %d %s for appeal %s", amount_minor, currency, appeal_id)
    return txn


async def refund_market_fee(
    db: AsyncSession,
    market_id: UUID,
    amount_minor: int,
    currency: str = "USD",
) -> InsuranceFundTxn:
    """Return the fee to the market if the market was voided after fee collection."""
    txn = InsuranceFundTxn(
        market_id=market_id,
        txn_type=InsuranceTxnType.refund,
        amount_minor=amount_minor,
        currency=currency,
        description=f"Fee refund for voided market {market_id}",
    )
    db.add(txn)
    await db.commit()
    await db.refresh(txn)
    logger.info("Insurance fee refunded: %d %s for market %s", amount_minor, currency, market_id)
    return txn


async def get_fund_balance(db: AsyncSession, currency: str = "USD") -> int:
    """
    Calculate current fund balance for a given currency.
    Balance = sum(deposits) - sum(payouts) - sum(refunds)
    """
    stmt = select(
        InsuranceFundTxn.txn_type,
        func.sum(InsuranceFundTxn.amount_minor).label("total"),
    ).where(
        InsuranceFundTxn.currency == currency
    ).group_by(InsuranceFundTxn.txn_type)

    result = await db.execute(stmt)
    rows = result.all()

    totals: dict[str, int] = {row.txn_type: int(row.total or 0) for row in rows}
    balance = (
        totals.get(InsuranceTxnType.deposit, 0)
        - totals.get(InsuranceTxnType.payout, 0)
        - totals.get(InsuranceTxnType.refund, 0)
    )
    return max(balance, 0)


async def get_fund_summary(db: AsyncSession) -> dict:
    """Return fund balance across all currencies."""
    stmt = select(
        InsuranceFundTxn.currency,
        InsuranceFundTxn.txn_type,
        func.sum(InsuranceFundTxn.amount_minor).label("total"),
    ).group_by(InsuranceFundTxn.currency, InsuranceFundTxn.txn_type)

    result = await db.execute(stmt)
    rows = result.all()

    summary: dict[str, dict] = {}
    for row in rows:
        cur = row.currency
        if cur not in summary:
            summary[cur] = {"deposits": 0, "payouts": 0, "refunds": 0}
        if row.txn_type == InsuranceTxnType.deposit:
            summary[cur]["deposits"] = int(row.total or 0)
        elif row.txn_type == InsuranceTxnType.payout:
            summary[cur]["payouts"] = int(row.total or 0)
        elif row.txn_type == InsuranceTxnType.refund:
            summary[cur]["refunds"] = int(row.total or 0)

    for cur, data in summary.items():
        data["balance"] = max(
            data["deposits"] - data["payouts"] - data["refunds"], 0
        )

    return summary
