# Responsible Gambling Features - Testing Guide

This document covers testing for Phase 3 P2 responsible gambling (RG) feature implementation.

## Overview

Responsible Gambling features include:
- Deposit limits (daily/monthly)
- Cool-off periods (24h, 7d, 30d)
- Self-exclusion (temporary or permanent)
- Loss streak notifications (user-configurable 1-10 threshold)
- Risk warnings (informational banners)

## Backend Testing

### Go Unit Tests (Wallet Service)

**File**: `services/wallet-service/internal/service/rg_service_test.go`

Tests domain logic without requiring external services:

```bash
# Run RG feature tests
cd services/wallet-service
go test -v ./internal/service -run TestDeposit
go test -v ./internal/service -run TestExclusion
go test -v ./internal/service -run TestOrderEligibility
go test -v ./internal/service -run TestCountryRGPolicy

# Run all tests
go test -v ./...
```

**Test Coverage**:
- ✅ DepositSettings domain logic (enabled, limit enforcement)
- ✅ ExclusionSettings domain logic (IsInCoolOff, CoolOffRemaining, IsPermanentlySelfExcluded)
- ✅ DailyDepositTracking logic (within/exceed limit)
- ✅ CountryRGPolicy logic (cool-off cancellation by region)
- ✅ OrderEligibility logic (cool-off + self-exclusion checks)

### Go Integration Tests (Wallet Service)

**Requirements**: PostgreSQL 16, Redis 8, Kafka

```bash
# Full integration test (requires external services)
# These would test the complete flow: deposit → limit check → gRPC call → DB persistence
cd services/wallet-service
docker-compose up -d  # Start test deps
go test -v -tags=integration ./...
docker-compose down
```

### Python Unit Tests (Resolution Service)

**File**: `services/resolution-service/resolution_service/test_tasks.py`

Tests loss tracking business logic:

```bash
# Run loss tracking tests
cd services/resolution-service
python -m pytest resolution_service/test_tasks.py -v

# Run specific test class
python -m pytest resolution_service/test_tasks.py::TestLossTrackingLogic -v

# Run with coverage
python -m pytest resolution_service/test_tasks.py --cov=resolution_service --cov-report=html
```

**Test Coverage**:
- ✅ Loss detection (payout < stake)
- ✅ Win detection (payout >= stake)
- ✅ Loss streak threshold checking
- ✅ Settlement event processing & parsing
- ✅ Loss alert event format & serialization
- ✅ Consecutive loss counter (increment on loss, reset on win)
- ✅ Loss amount aggregation
- ✅ Timestamp handling (ISO format)

### Go RPC Tests (Wallet Service)

Test gRPC endpoint behavior:

```bash
# Compile proto first
cd services/wallet-service
protoc --go_out=. --go-grpc_out=. proto/wallet.proto

# These endpoints can be tested with grpcurl:
grpcurl -d '{"user_id":"550e8400-e29b-41d4-a716-446655440000"}' \
  localhost:9002 predictx.wallet.WalletService/GetDepositSettings
```

## Frontend Testing

### Mobile Integration

**File**: `apps/mobile/src/store/useRGStore.ts`

```bash
# Install dependencies
cd apps/mobile
npm install

# Run store tests (if configured)
npm test -- useRGStore
```

**Manual Testing Steps**:

1. **Deposit Limits**:
   - Navigate to RG Settings screen
   - Verify deposit limit is displayed (default $50)
   - Attempt deposit at limit boundary
   - Verify banner shows warning colors (red <10%, yellow 10-30%, green >30%)

2. **Cool-off Period**:
   - Click "24 Hours" / "7 Days" / "30 Days" button
   - Confirm in modal
   - Verify exclusion settings show cool-off active
   - Verify deposits are blocked (DepositRemainingBanner shows "⏸️ Cool-off Active")
   - Verify cancel button appears (if region allows)
   - For India: Test cancel functionality
   - For other regions: Verify cancel is disabled

3. **Self-Exclusion**:
   - Click "Self-Exclude Account"
   - Select temporary (7/30/90 days) or permanent
   - Confirm in modal
   - Verify orders are blocked
   - Verify banner shows "🛑 Self-Excluded"

4. **Loss Streak Alerts**:
   - Simulate 3+ consecutive losses via backend
   - Verify modal appears with consecutive count
   - Verify "Take a Break" button offers cool-off option
   - Verify partnership link visible

5. **WebSocket Connection**:
   - Open network tab in dev tools
   - Navigate to RG Settings
   - Verify WebSocket connects to `user:{userId}:rg_alerts` topic
   - Trigger loss streak alert from backend
   - Verify modal appears automatically (no refresh needed)

## API Testing

### Postman/curl Tests

**Deposit Settings**:
```bash
# Get current deposit settings
curl -X GET http://localhost:8002/v1/wallets/{userId}/deposit-settings \
  -H "Authorization: Bearer {token}"

# Update deposit limit
curl -X PUT http://localhost:8002/v1/wallets/{userId}/deposit-settings \
  -H "Authorization: Bearer {token}" \
  -d '{"daily_limit_minor": 10000000, "monthly_limit_minor": 300000000}'
```

**Cool-off**:
```bash
# Start cool-off (24 hours)
curl -X POST http://localhost:8002/v1/wallets/{userId}/cool-off \
  -H "Authorization: Bearer {token}" \
  -d '{"duration_hours": 24}'

# Get current exclusion settings
curl -X GET http://localhost:8002/v1/wallets/{userId}/exclusion-settings \
  -H "Authorization: Bearer {token}"

# Cancel cool-off (if allowed by region)
curl -X DELETE http://localhost:8002/v1/wallets/{userId}/cool-off \
  -H "Authorization: Bearer {token}"
```

**Self-Exclusion**:
```bash
# Self-exclude temporarily (30 days)
curl -X POST http://localhost:8002/v1/wallets/{userId}/self-exclude \
  -H "Authorization: Bearer {token}" \
  -d '{"duration_days": 30}'

# Self-exclude permanently
curl -X POST http://localhost:8002/v1/wallets/{userId}/self-exclude \
  -H "Authorization: Bearer {token}" \
  -d '{"duration_days": null}'
```

**Order Eligibility**:
```bash
# Check if user can place an order
curl -X GET http://localhost:8002/v1/wallets/{userId}/order-eligibility \
  -H "Authorization: Bearer {token}"

# Returns: {eligible: bool, reason: string}
# Reasons: "in_cool_off_period" | "self_excluded" | ""
```

**Loss Streak Threshold**:
```bash
# Get current threshold
curl -X GET http://localhost:8002/v1/wallets/{userId}/loss-streak-threshold \
  -H "Authorization: Bearer {token}"

# Update threshold (1-10)
curl -X PUT http://localhost:8002/v1/wallets/{userId}/loss-streak-threshold \
  -H "Authorization: Bearer {token}" \
  -d '{"threshold": 5}'
```

## Kafka Event Testing

### Loss Streak Alerts

```bash
# Monitor Kafka for loss streak alerts
kafka-console-consumer.sh \
  --bootstrap-servers localhost:9092 \
  --topic user.loss_streak_alert \
  --from-beginning \
  --property print.timestamp=true

# Sample event payload:
# {
#   "user_id": "550e8400-e29b-41d4-a716-446655440000",
#   "consecutive_losses": 3,
#   "market_ids": ["uuid1", "uuid2", "uuid3"],
#   "total_loss_minor": 50000,
#   "timestamp": "2026-03-10T10:30:45.123Z"
# }
```

## Database Testing

### Verify Schema

```bash
# Connect to PostgreSQL
psql -h localhost -U postgres -d predictx

# Check deposit settings table
SELECT * FROM user_deposit_settings WHERE user_id = '{userId}';

# Check exclusion settings
SELECT * FROM user_exclusion_settings WHERE user_id = '{userId}';

# Check daily tracking
SELECT * FROM daily_deposit_tracking WHERE user_id = '{userId}';

# Check loss tracking
SELECT * FROM user_loss_tracking WHERE user_id = '{userId}';

# Check country policies
SELECT * FROM country_rg_policy;
```

## Environment Configuration

**Required env vars** for full RG feature functionality:

```bash
# Wallet Service
ENABLE_RG_FEATURES=true
DEFAULT_DAILY_DEPOSIT_LIMIT_MINOR=5000000      # $50
DEFAULT_MONTHLY_DEPOSIT_LIMIT_MINOR=150000000  # $1500
DEFAULT_LOSS_STREAK_NOTIFICATION_THRESHOLD=3
RG_PARTNERSHIP_NAME="Gambler's Anonymous"
COOL_OFF_CANCELLATION_POLICY='{"IN": true, "NG": false, "KE": false, "PH": false}'

# Order Service (must call wallet-service)
WALLET_SERVICE_GRPC_URL=localhost:9002

# Resolution Service (loss tracking)
KAFKA_BROKERS=localhost:9092
```

## Validation Checklist

- [ ] Deposit limits enforced (daily/monthly)
- [ ] Cool-off blocks new deposits
- [ ] Cool-off cancellation works only in allowed regions (India)
- [ ] Self-exclusion blocks all betting & deposits
- [ ] Loss streak alerts triggered at user-configured threshold
- [ ] WebSocket alerts delivered in real-time (<50ms)
- [ ] Order service checks eligibility before placement
- [ ] All gRPC endpoints return correct error codes
- [ ] Database migrations execute cleanly
- [ ] Config validation catches missing RG settings
- [ ] Timestamps in UTC/ISO format
- [ ] Country policy enforcement working

## Troubleshooting

**Issue**: Cool-off cancel button not appearing
- **Solution**: Check user's country_code matches config; verify COOL_OFF_CANCELLATION_POLICY env var is set

**Issue**: Loss alerts not received
- **Solution**: Verify WebSocket connection to `user:{userId}:rg_alerts`; check Kafka consumer is running; verify settlement.completed events are being published

**Issue**: Deposit limit not enforced
- **Solution**: Ensure ENABLE_RG_FEATURES=true; check daily_deposit_tracking table is being updated; verify checkDepositEligibility() is called in Deposit() flow

**Issue**: Order eligibility check fails
- **Solution**: Verify wallet-service is running on port 9002; check gRPC connection in order-service logs; ensure JWT token is valid

## References

- **RG Implementation Plan**: See earlier plan mode discussion
- **Domain Models**: `services/wallet-service/internal/domain/`
- **Service Layer**: `services/wallet-service/internal/service/wallet_service.go`
- **gRPC Protos**: `services/wallet-service/proto/wallet.proto`
- **Frontend Store**: `apps/mobile/src/store/useRGStore.ts`
- **Loss Tracking**: `services/resolution-service/resolution_service/tasks.py`
