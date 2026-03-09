use uuid::Uuid;

use crate::domain::order::{Order, OrderType};
use crate::domain::trade::Trade;
use crate::engine::amm::LmsrAmm;
use crate::engine::order_book::OrderBook;

/// Process a single incoming order against the book and AMM.
///
/// Strategy:
///   1. Add order to CLOB
///   2. Run CLOB matching (price-time priority)
///   3. For any remaining quantity on limit orders: try AMM fallback
///   4. For market orders with no CLOB match: fill fully against AMM
///
/// Returns all generated trades (CLOB + AMM).
pub fn process_order(book: &mut OrderBook, amm: &mut LmsrAmm, order: Order) -> Vec<Trade> {
    let market_id = order.market_id;
    let user_id = order.user_id;
    let outcome_index = order.outcome_index;
    let is_market_order = order.order_type == OrderType::Market;
    let limit_price = order.price_minor;
    let original_qty = order.remaining;

    // Step 1: add to CLOB
    book.add_order(order);

    // Step 2: CLOB matching
    let mut trades = book.match_orders();

    let clob_filled: u64 = trades.iter().map(|t| t.quantity).sum();
    let remaining = original_qty.saturating_sub(clob_filled);

    if remaining == 0 {
        return trades;
    }

    // Step 3: AMM fallback for unmatched quantity
    let effective_price = if is_market_order { 99 } else { limit_price };

    if amm.can_fill(outcome_index, effective_price) {
        let seq_no = book.current_seq() + 1;
        let amm_trade = fill_amm(amm, market_id, user_id, outcome_index, remaining, seq_no);
        if let Some(t) = amm_trade {
            // Remove the unfilled remainder from the book since AMM filled it
            // (The remaining order in the book was for `remaining` qty; AMM filled it)
            // We do a best-effort cancel of the most recent unmatched order
            book.cancel_all_user_orders_for_market(user_id, market_id, outcome_index);
            trades.push(t);
        }
    }

    trades
}

fn fill_amm(
    amm: &mut LmsrAmm,
    market_id: Uuid,
    user_id: Uuid,
    outcome_index: usize,
    quantity: u64,
    seq_no: u64,
) -> Option<Trade> {
    if quantity == 0 {
        return None;
    }
    let (avg_price, _new_odds) = amm.fill_order(outcome_index, quantity as f64);
    Some(Trade::new_amm(
        market_id,
        user_id,
        avg_price,
        quantity,
        outcome_index,
        seq_no,
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::order::{OrderSide, OrderType};
    use chrono::Utc;

    fn make_order(side: OrderSide, price: u64, qty: u64, market_id: Uuid) -> Order {
        Order {
            id: Uuid::new_v4(),
            market_id,
            user_id: Uuid::new_v4(),
            side,
            order_type: OrderType::Limit,
            price_minor: price,
            quantity: qty,
            remaining: qty,
            outcome_index: 0,
            seq_no: 0,
            idempotency_key: Uuid::new_v4().to_string(),
            placed_at: Utc::now(),
        }
    }

    #[test]
    fn clob_match_two_orders() {
        let market_id = Uuid::new_v4();
        let mut book = OrderBook::new();
        let mut amm = LmsrAmm::new(100.0, 2);

        process_order(&mut book, &mut amm, make_order(OrderSide::Yes, 70, 100, market_id));
        let trades = process_order(&mut book, &mut amm, make_order(OrderSide::No, 30, 100, market_id));

        assert_eq!(trades.len(), 1);
        assert_eq!(trades[0].quantity, 100);
        assert!(matches!(trades[0].match_type, crate::domain::trade::MatchType::Clob));
    }

    #[test]
    fn amm_fallback_when_no_clob_match() {
        let market_id = Uuid::new_v4();
        let mut book = OrderBook::new();
        let mut amm = LmsrAmm::new(100.0, 2);

        // Market order — no counter-order in CLOB → should go to AMM
        let mut order = make_order(OrderSide::Yes, 99, 50, market_id);
        order.order_type = OrderType::Market;
        let trades = process_order(&mut book, &mut amm, order);

        assert_eq!(trades.len(), 1);
        assert!(matches!(trades[0].match_type, crate::domain::trade::MatchType::Amm));
    }

    #[test]
    fn no_amm_when_price_too_low() {
        let market_id = Uuid::new_v4();
        let mut book = OrderBook::new();
        let mut amm = LmsrAmm::new(100.0, 2); // initial odds ~50

        // Limit order at price 1 — AMM price is ~50, can't fill at 1
        let order = make_order(OrderSide::Yes, 1, 50, market_id);
        let trades = process_order(&mut book, &mut amm, order);
        assert!(trades.is_empty());
    }
}
