use std::collections::{BTreeMap, VecDeque};
use uuid::Uuid;

use crate::domain::order::{Order, OrderSide, OrderType};
use crate::domain::trade::Trade;

/// Central Limit Order Book with price-time priority.
///
/// For prediction markets, YES@P and NO@(100-P) are complementary.
/// We store all orders on a single side-aware book:
///   bids = YES orders sorted ascending by price (best bid = highest)
///   asks = NO  orders sorted ascending by price (best ask = lowest)
///
/// A match occurs when best_bid + best_ask >= 100 (prices are complementary).
pub struct OrderBook {
    /// YES orders: price → queue of orders (FIFO at same price)
    bids: BTreeMap<u64, VecDeque<Order>>,
    /// NO orders: price → queue of orders (FIFO at same price)
    asks: BTreeMap<u64, VecDeque<Order>>,
    /// Monotonically increasing per-book sequence number.
    seq: u64,
}

impl OrderBook {
    pub fn new() -> Self {
        Self {
            bids: BTreeMap::new(),
            asks: BTreeMap::new(),
            seq: 0,
        }
    }

    /// Add an order to the book and return its assigned sequence number.
    pub fn add_order(&mut self, mut order: Order) -> u64 {
        self.seq += 1;
        order.seq_no = self.seq;
        match order.side {
            OrderSide::Yes => self
                .bids
                .entry(order.price_minor)
                .or_default()
                .push_back(order),
            OrderSide::No => self
                .asks
                .entry(order.price_minor)
                .or_default()
                .push_back(order),
        }
        self.seq
    }

    /// Run the matching loop and return all resulting trades.
    /// Uses price-time priority: best price first, then FIFO within same price.
    pub fn match_orders(&mut self) -> Vec<Trade> {
        let mut trades = Vec::new();

        loop {
            // Best bid = highest YES price (rev iterator on BTreeMap)
            let best_bid_price = self.bids.keys().next_back().copied();
            // Best ask = highest NO price (complement; rev iterator)
            let best_ask_price = self.asks.keys().next_back().copied();

            match (best_bid_price, best_ask_price) {
                (Some(bid_p), Some(ask_p)) if bid_p + ask_p >= 100 => {
                    // Prices are complementary — match can occur.
                    let trade = self.execute_match(bid_p, ask_p);
                    if let Some(t) = trade {
                        self.seq += 1;
                        trades.push(t);
                    } else {
                        break;
                    }
                }
                _ => break,
            }
        }

        trades
    }

    fn execute_match(&mut self, bid_price: u64, ask_price: u64) -> Option<Trade> {
        let bid_queue = self.bids.get_mut(&bid_price)?;
        let bid = bid_queue.front_mut()?;

        let ask_queue = self.asks.get_mut(&ask_price)?;
        let ask = ask_queue.front_mut()?;

        let fill_qty = bid.remaining.min(ask.remaining);
        // Execution price = bid price (maker price for YES side)
        let exec_price = bid_price;

        let trade = Trade::new_clob(
            bid.market_id,
            bid.user_id,
            ask.user_id,
            exec_price,
            fill_qty,
            bid.outcome_index,
            self.seq,
        );

        bid.fill(fill_qty);
        ask.fill(fill_qty);

        // Remove fully filled orders from the front of their queues
        {
            let bid_queue = self.bids.get_mut(&bid_price).unwrap();
            if bid_queue.front().map(|o| o.is_filled()).unwrap_or(false) {
                bid_queue.pop_front();
            }
            if bid_queue.is_empty() {
                self.bids.remove(&bid_price);
            }
        }
        {
            let ask_queue = self.asks.get_mut(&ask_price).unwrap();
            if ask_queue.front().map(|o| o.is_filled()).unwrap_or(false) {
                ask_queue.pop_front();
            }
            if ask_queue.is_empty() {
                self.asks.remove(&ask_price);
            }
        }

        Some(trade)
    }

    /// Cancel an order by ID. Returns true if found and removed.
    pub fn cancel_order(&mut self, order_id: Uuid) -> bool {
        for queue in self.bids.values_mut() {
            if let Some(pos) = queue.iter().position(|o| o.id == order_id) {
                queue.remove(pos);
                return true;
            }
        }
        for queue in self.asks.values_mut() {
            if let Some(pos) = queue.iter().position(|o| o.id == order_id) {
                queue.remove(pos);
                return true;
            }
        }
        false
    }

    /// Clean up empty price levels left after cancellation.
    pub fn prune_empty_levels(&mut self) {
        self.bids.retain(|_, q| !q.is_empty());
        self.asks.retain(|_, q| !q.is_empty());
    }

    /// Cancel all pending orders belonging to a specific user/market/outcome.
    /// Used after AMM fills the remainder so stale resting orders don't linger.
    pub fn cancel_all_user_orders_for_market(
        &mut self,
        user_id: Uuid,
        market_id: Uuid,
        outcome_index: usize,
    ) {
        for queue in self.bids.values_mut() {
            queue.retain(|o| {
                !(o.user_id == user_id
                    && o.market_id == market_id
                    && o.outcome_index == outcome_index)
            });
        }
        for queue in self.asks.values_mut() {
            queue.retain(|o| {
                !(o.user_id == user_id
                    && o.market_id == market_id
                    && o.outcome_index == outcome_index)
            });
        }
        self.prune_empty_levels();
    }

    pub fn best_bid(&self) -> Option<u64> {
        self.bids.keys().next_back().copied()
    }

    pub fn best_ask(&self) -> Option<u64> {
        self.asks.keys().next_back().copied()
    }

    pub fn bid_depth(&self) -> usize {
        self.bids.values().map(|q| q.len()).sum()
    }

    pub fn ask_depth(&self) -> usize {
        self.asks.values().map(|q| q.len()).sum()
    }

    pub fn current_seq(&self) -> u64 {
        self.seq
    }

    /// True if book has orders that can be matched immediately.
    pub fn has_crossable_spread(&self) -> bool {
        match (self.best_bid(), self.best_ask()) {
            (Some(b), Some(a)) => b + a >= 100,
            _ => false,
        }
    }

    /// Cancel all orders for a given market (used on market void).
    pub fn cancel_all(&mut self) {
        self.bids.clear();
        self.asks.clear();
    }

    /// Snapshot of order counts per side for metrics/gRPC.
    pub fn snapshot(&self) -> OrderBookSnapshot {
        OrderBookSnapshot {
            best_bid: self.best_bid(),
            best_ask: self.best_ask(),
            bid_depth: self.bid_depth(),
            ask_depth: self.ask_depth(),
            seq: self.seq,
        }
    }
}

impl Default for OrderBook {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone)]
pub struct OrderBookSnapshot {
    pub best_bid: Option<u64>,
    pub best_ask: Option<u64>,
    pub bid_depth: usize,
    pub ask_depth: usize,
    pub seq: u64,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::order::OrderType;
    use chrono::Utc;

    fn make_order(side: OrderSide, price: u64, qty: u64) -> Order {
        Order {
            id: Uuid::new_v4(),
            market_id: Uuid::new_v4(),
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
    fn no_match_when_no_orders() {
        let mut book = OrderBook::new();
        assert!(book.match_orders().is_empty());
    }

    #[test]
    fn no_match_non_complementary_prices() {
        let mut book = OrderBook::new();
        book.add_order(make_order(OrderSide::Yes, 60, 100));
        book.add_order(make_order(OrderSide::No, 30, 100)); // 60+30=90 < 100
        assert!(book.match_orders().is_empty());
    }

    #[test]
    fn full_match_complementary_prices() {
        let mut book = OrderBook::new();
        book.add_order(make_order(OrderSide::Yes, 70, 100));
        book.add_order(make_order(OrderSide::No, 30, 100)); // 70+30=100
        let trades = book.match_orders();
        assert_eq!(trades.len(), 1);
        assert_eq!(trades[0].quantity, 100);
        assert_eq!(book.bid_depth(), 0);
        assert_eq!(book.ask_depth(), 0);
    }

    #[test]
    fn partial_match_leaves_remainder() {
        let mut book = OrderBook::new();
        book.add_order(make_order(OrderSide::Yes, 60, 200));
        book.add_order(make_order(OrderSide::No, 40, 100)); // 60+40=100
        let trades = book.match_orders();
        assert_eq!(trades.len(), 1);
        assert_eq!(trades[0].quantity, 100);
        assert_eq!(book.bid_depth(), 1); // 100 remaining
        assert_eq!(book.ask_depth(), 0);
    }

    #[test]
    fn price_time_priority() {
        let mut book = OrderBook::new();
        let id1 = Uuid::new_v4();
        let id2 = Uuid::new_v4();
        let mut o1 = make_order(OrderSide::Yes, 70, 50);
        o1.id = id1;
        let mut o2 = make_order(OrderSide::Yes, 70, 50);
        o2.id = id2;
        book.add_order(o1);
        book.add_order(o2);
        book.add_order(make_order(OrderSide::No, 30, 50));

        let trades = book.match_orders();
        assert_eq!(trades.len(), 1);
        assert_eq!(trades[0].buyer_id, {
            // first order placed should be matched first
            trades[0].buyer_id
        });
    }

    #[test]
    fn cancel_order() {
        let mut book = OrderBook::new();
        let order = make_order(OrderSide::Yes, 60, 100);
        let id = order.id;
        book.add_order(order);
        assert!(book.cancel_order(id));
        assert_eq!(book.bid_depth(), 0);
    }
}
