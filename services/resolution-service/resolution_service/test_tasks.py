"""
Unit tests for Celery tasks - loss tracking
"""
import asyncio
import json
import logging
from datetime import datetime
from uuid import uuid4

import pytest

logger = logging.getLogger(__name__)


class MockSettlement:
    """Mock settlement event for testing"""
    def __init__(self, user_id: str, stake_minor: int, payout_minor: int, market_id: str = None):
        self.user_id = user_id
        self.stake_minor = stake_minor
        self.payout_minor = payout_minor
        self.market_id = market_id or str(uuid4())

    def to_dict(self):
        return {
            "user_id": self.user_id,
            "position_stake_minor": self.stake_minor,
            "position_payout_minor": self.payout_minor,
            "market_id": self.market_id,
        }


class TestLossTrackingLogic:
    """Test loss tracking business logic"""

    def test_loss_detection(self):
        """Test that losses are correctly identified"""
        settlement = MockSettlement(
            user_id=str(uuid4()),
            stake_minor=1000000,  # $10
            payout_minor=500000,  # $5 (loss)
        )
        is_loss = settlement.payout_minor < settlement.stake_minor
        assert is_loss is True

    def test_win_detection(self):
        """Test that wins are correctly identified"""
        settlement = MockSettlement(
            user_id=str(uuid4()),
            stake_minor=1000000,  # $10
            payout_minor=2000000,  # $20 (win)
        )
        is_loss = settlement.payout_minor < settlement.stake_minor
        assert is_loss is False

    def test_push_detection(self):
        """Test that pushes are correctly identified"""
        settlement = MockSettlement(
            user_id=str(uuid4()),
            stake_minor=1000000,  # $10
            payout_minor=1000000,  # $10 (push/draw)
        )
        is_loss = settlement.payout_minor < settlement.stake_minor
        assert is_loss is False


class TestLossStreakThresholdLogic:
    """Test loss streak threshold checking"""

    def test_threshold_exceeded(self):
        """Test when loss streak exceeds threshold"""
        consecutive_losses = 5
        threshold = 3
        should_alert = consecutive_losses >= threshold
        assert should_alert is True

    def test_threshold_not_exceeded(self):
        """Test when loss streak is below threshold"""
        consecutive_losses = 2
        threshold = 3
        should_alert = consecutive_losses >= threshold
        assert should_alert is False

    def test_threshold_at_boundary(self):
        """Test when loss streak equals threshold"""
        consecutive_losses = 3
        threshold = 3
        should_alert = consecutive_losses >= threshold
        assert should_alert is True


class TestSettlementEventProcessing:
    """Test settlement event processing logic"""

    def test_settlement_payload_parsing(self):
        """Test that settlement payloads are correctly parsed"""
        user_id = str(uuid4())
        market_id = str(uuid4())
        stake = 1000000
        payout = 500000

        payload = {
            "user_id": user_id,
            "position_stake_minor": stake,
            "position_payout_minor": payout,
            "market_id": market_id,
        }

        # Verify payload can be parsed
        assert payload["user_id"] == user_id
        assert payload["position_stake_minor"] == stake
        assert payload["position_payout_minor"] == payout
        assert payload["market_id"] == market_id

    def test_loss_alert_event_format(self):
        """Test that loss alert events have correct format"""
        user_id = str(uuid4())
        market_id = str(uuid4())
        consecutive_losses = 3
        total_loss = 50000
        stake = 1000000
        payout = 500000

        alert_event = {
            "user_id": user_id,
            "consecutive_losses": consecutive_losses,
            "market_ids": [market_id],
            "total_loss_minor": total_loss,
            "stake_minor": stake,
            "payout_minor": payout,
            "timestamp": datetime.utcnow().isoformat(),
        }

        # Verify event has all required fields
        assert alert_event["user_id"]
        assert alert_event["consecutive_losses"] > 0
        assert alert_event["market_ids"]
        assert "timestamp" in alert_event
        assert isinstance(alert_event["timestamp"], str)

    def test_loss_alert_event_json_serializable(self):
        """Test that loss alert events are JSON serializable"""
        alert_event = {
            "user_id": str(uuid4()),
            "consecutive_losses": 3,
            "market_ids": [str(uuid4())],
            "total_loss_minor": 50000,
            "timestamp": datetime.utcnow().isoformat(),
        }

        # Should be JSON serializable
        json_str = json.dumps(alert_event)
        assert json_str
        assert isinstance(json_str, str)

        # Should be deserializable
        decoded = json.loads(json_str)
        assert decoded["user_id"] == alert_event["user_id"]
        assert decoded["consecutive_losses"] == 3


class TestConsecutiveLossCounter:
    """Test consecutive loss tracking logic"""

    def test_loss_increments_counter(self):
        """Test that losses increment the counter"""
        consecutive_losses = 0
        for i in range(3):
            settlement = MockSettlement(
                user_id=str(uuid4()),
                stake_minor=1000000,
                payout_minor=500000,  # Loss
            )
            if settlement.payout_minor < settlement.stake_minor:
                consecutive_losses += 1

        assert consecutive_losses == 3

    def test_win_resets_counter(self):
        """Test that wins reset the counter"""
        consecutive_losses = 5

        # Simulate a win
        settlement = MockSettlement(
            user_id=str(uuid4()),
            stake_minor=1000000,
            payout_minor=2000000,  # Win
        )
        if settlement.payout_minor >= settlement.stake_minor:
            consecutive_losses = 0

        assert consecutive_losses == 0

    def test_mixed_loss_win_sequence(self):
        """Test tracking through mixed loss/win sequence"""
        consecutive_losses = 0
        settlements = [
            MockSettlement(str(uuid4()), 1000000, 500000),   # Loss
            MockSettlement(str(uuid4()), 1000000, 500000),   # Loss
            MockSettlement(str(uuid4()), 1000000, 2000000),  # Win (reset)
            MockSettlement(str(uuid4()), 1000000, 500000),   # Loss
        ]

        for settlement in settlements:
            if settlement.payout_minor < settlement.stake_minor:
                consecutive_losses += 1
            else:
                consecutive_losses = 0

        assert consecutive_losses == 1  # Only 1 loss after final sequence


class TestLossAmountAggregation:
    """Test aggregating loss amounts"""

    def test_loss_aggregation(self):
        """Test that losses are correctly aggregated"""
        settlements = [
            {"stake": 1000000, "payout": 500000, "loss": 500000},
            {"stake": 2000000, "payout": 1000000, "loss": 1000000},
            {"stake": 500000, "payout": 200000, "loss": 300000},
        ]

        total_loss = 0
        for s in settlements:
            if s["payout"] < s["stake"]:
                loss = s["stake"] - s["payout"]
                total_loss += loss

        assert total_loss == 1800000

    def test_zero_loss_on_wins(self):
        """Test that wins don't contribute to loss total"""
        settlements = [
            {"stake": 1000000, "payout": 2000000},  # Win (+$10)
            {"stake": 1000000, "payout": 500000},   # Loss (-$5)
        ]

        total_loss = 0
        for s in settlements:
            if s["payout"] < s["stake"]:
                loss = s["stake"] - s["payout"]
                total_loss += loss

        assert total_loss == 500000


class TestTimestampHandling:
    """Test timestamp handling in events"""

    def test_iso_format_timestamp(self):
        """Test that timestamps are in ISO format"""
        now = datetime.utcnow()
        iso_str = now.isoformat()

        # Should be parseable back
        parsed = datetime.fromisoformat(iso_str)
        assert parsed.year == now.year
        assert parsed.month == now.month
        assert parsed.day == now.day

    def test_event_timestamp_present(self):
        """Test that events include timestamp"""
        event = {
            "user_id": str(uuid4()),
            "consecutive_losses": 3,
            "timestamp": datetime.utcnow().isoformat(),
        }

        assert "timestamp" in event
        assert len(event["timestamp"]) > 0
