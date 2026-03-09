/**
 * DepositRemainingBanner
 * Shows daily deposit remaining budget with color-coded warning levels
 */
import React from 'react';
import { View, StyleSheet } from 'react-native';
import { Text } from './Text';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { useRGStore } from '@/store/useRGStore';

const styles = StyleSheet.create({
  banner: {
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[3],
    borderRadius: 8,
    marginBottom: Spacing[4],
  },
  bannerGreen: {
    backgroundColor: 'rgba(34, 197, 94, 0.1)',
    borderLeftWidth: 4,
    borderLeftColor: '#22c55e',
  },
  bannerYellow: {
    backgroundColor: 'rgba(234, 179, 8, 0.1)',
    borderLeftWidth: 4,
    borderLeftColor: '#eab308',
  },
  bannerRed: {
    backgroundColor: 'rgba(239, 68, 68, 0.1)',
    borderLeftWidth: 4,
    borderLeftColor: '#ef4444',
  },
  title: {
    fontSize: 12,
    fontWeight: '600',
    color: Colors.text2,
    marginBottom: Spacing[1],
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  content: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  remaining: {
    fontSize: 16,
    fontWeight: '700',
    color: Colors.text,
  },
  percentage: {
    fontSize: 14,
    fontWeight: '500',
    color: Colors.text2,
  },
  progressBar: {
    height: 4,
    backgroundColor: Colors.border,
    borderRadius: 2,
    marginTop: Spacing[2],
    overflow: 'hidden',
  },
  progress: {
    height: '100%',
    borderRadius: 2,
  },
  progressGreen: {
    backgroundColor: '#22c55e',
  },
  progressYellow: {
    backgroundColor: '#eab308',
  },
  progressRed: {
    backgroundColor: '#ef4444',
  },
});

interface DepositRemainingBannerProps {
  visible?: boolean;
}

export const DepositRemainingBanner: React.FC<DepositRemainingBannerProps> = ({ visible = true }) => {
  const depositSettings = useRGStore((state) => state.depositSettings);
  const exclusionSettings = useRGStore((state) => state.exclusionSettings);

  if (!visible || !depositSettings) return null;

  const dailyLimit = depositSettings.dailyDepositLimitMinor;
  const remaining = Math.max(0, depositSettings.remainingDailyBudget ?? dailyLimit);
  const used = dailyLimit - remaining;
  const percentageRemaining = dailyLimit > 0 ? (remaining / dailyLimit) * 100 : 0;

  // Determine banner color based on remaining percentage
  let bannerStyle = styles.bannerGreen;
  let progressStyle = styles.progressGreen;
  let warningText = 'Deposit limit healthy';

  if (percentageRemaining < 10) {
    bannerStyle = styles.bannerRed;
    progressStyle = styles.progressRed;
    warningText = 'Near daily limit!';
  } else if (percentageRemaining < 30) {
    bannerStyle = styles.bannerYellow;
    progressStyle = styles.progressYellow;
    warningText = 'Approaching daily limit';
  }

  // Don't show if in cool-off
  if (exclusionSettings?.inCoolOff) {
    return (
      <View style={[styles.banner, styles.bannerYellow]}>
        <Text style={styles.title}>⏸️ Cool-off Active</Text>
        <Text style={styles.percentage}>
          You are in a cool-off period. Deposits are temporarily unavailable.
        </Text>
      </View>
    );
  }

  // Don't show if self-excluded
  if (exclusionSettings?.isSelfExcluded) {
    return (
      <View style={[styles.banner, styles.bannerRed]}>
        <Text style={styles.title}>🛑 Self-Excluded</Text>
        <Text style={styles.percentage}>
          Your account is self-excluded. Please contact support to reactivate.
        </Text>
      </View>
    );
  }

  const displayRemaining = remaining > 0 ? `$${(remaining / 1000000).toFixed(2)}` : '$0.00';
  const displayLimit = `$${(dailyLimit / 1000000).toFixed(2)}`;

  return (
    <View style={[styles.banner, bannerStyle]}>
      <Text style={styles.title}>{warningText}</Text>
      <View style={styles.content}>
        <Text style={styles.remaining}>{displayRemaining} remaining</Text>
        <Text style={styles.percentage}>of {displayLimit}</Text>
      </View>
      <View style={styles.progressBar}>
        <View
          style={[
            styles.progress,
            progressStyle,
            { width: `${Math.min(100, percentageRemaining)}%` },
          ]}
        />
      </View>
    </View>
  );
};
