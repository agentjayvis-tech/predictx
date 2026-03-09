/**
 * MarketCard Component
 * Displays a single market with swipe gestures
 */
import React, { useState } from 'react';
import {
  View,
  StyleSheet,
  TouchableOpacity,
  GestureResponderEvent,
} from 'react-native';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { Text } from './Text';
import { Market } from '@/store/useMarketStore';

interface MarketCardProps {
  market: Market;
  onSwipeLeft?: () => void;
  onSwipeRight?: () => void;
  onPress?: () => void;
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: Colors.surface2,
    borderRadius: 16,
    padding: Spacing[4],
    marginBottom: Spacing[3],
    borderWidth: 1,
    borderColor: Colors.border,
  },
  header: {
    marginBottom: Spacing[3],
  },
  question: {
    marginBottom: Spacing[2],
  },
  category: {
    fontSize: 12,
    color: Colors.text2,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
    marginBottom: Spacing[2],
  },
  outcomes: {
    flexDirection: 'row',
    gap: Spacing[2],
    marginBottom: Spacing[3],
  },
  outcomeButton: {
    flex: 1,
    borderRadius: 8,
    paddingVertical: Spacing[3],
    paddingHorizontal: Spacing[2],
    alignItems: 'center',
    borderWidth: 1,
    borderColor: Colors.border,
  },
  yesButton: {
    backgroundColor: Colors.greenDim,
  },
  noButton: {
    backgroundColor: Colors.redDim,
  },
  outcomeLabel: {
    fontSize: 12,
    marginBottom: Spacing[1],
  },
  odds: {
    fontSize: 16,
  },
  footer: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingTop: Spacing[3],
    borderTopWidth: 1,
    borderTopColor: Colors.border,
  },
  time: {
    fontSize: 12,
    color: Colors.text2,
  },
  volume: {
    fontSize: 12,
    color: Colors.accent,
  },
});

export const MarketCard: React.FC<MarketCardProps> = ({
  market,
  onSwipeLeft,
  onSwipeRight,
  onPress,
}) => {
  const [startX, setStartX] = useState(0);

  const handleTouchStart = (event: GestureResponderEvent) => {
    setStartX(event.nativeEvent.pageX);
  };

  const handleTouchEnd = (event: GestureResponderEvent) => {
    const endX = event.nativeEvent.pageX;
    const diff = startX - endX;

    if (Math.abs(diff) > 50) {
      if (diff > 0) {
        onSwipeLeft?.();
      } else {
        onSwipeRight?.();
      }
    }
  };

  const outcome = market.outcomes[0];

  return (
    <TouchableOpacity
      onPress={onPress}
      onTouchStart={handleTouchStart}
      onTouchEnd={handleTouchEnd}
      activeOpacity={0.7}
    >
      <View style={styles.card}>
        <View style={styles.header}>
          <Text style={styles.category}>{market.category}</Text>
          <Text variant="h3" style={styles.question}>
            {market.question}
          </Text>
        </View>

        <View style={styles.outcomes}>
          <TouchableOpacity
            style={[styles.outcomeButton, styles.yesButton]}
          >
            <Text style={styles.outcomeLabel} color={Colors.text2}>
              YES
            </Text>
            <Text style={styles.odds} color={Colors.green} weight="bold">
              {outcome?.yesOdds.toFixed(2)}
            </Text>
          </TouchableOpacity>

          <TouchableOpacity
            style={[styles.outcomeButton, styles.noButton]}
          >
            <Text style={styles.outcomeLabel} color={Colors.text2}>
              NO
            </Text>
            <Text style={styles.odds} color={Colors.red} weight="bold">
              {outcome?.noOdds.toFixed(2)}
            </Text>
          </TouchableOpacity>
        </View>

        <View style={styles.footer}>
          <Text style={styles.time} color={Colors.text2}>
            Closes in {new Date(market.closesAt).toLocaleDateString()}
          </Text>
          <Text style={styles.volume} color={Colors.accent}>
            ₹{(market.volume / 1000).toFixed(0)}K
          </Text>
        </View>
      </View>
    </TouchableOpacity>
  );
};
