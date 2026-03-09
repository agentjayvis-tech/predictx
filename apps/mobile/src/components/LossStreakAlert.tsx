/**
 * LossStreakAlert
 * Modal showing loss streak notification with option to take a break
 */
import React from 'react';
import {
  View,
  StyleSheet,
  Modal,
  Pressable,
  ScrollView,
} from 'react-native';
import { Text } from './Text';
import { Button } from './Button';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { useRGStore } from '@/store/useRGStore';

const styles = StyleSheet.create({
  modal: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
    justifyContent: 'flex-end',
  },
  container: {
    backgroundColor: Colors.bg,
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[6],
    paddingBottom: Spacing[8],
  },
  header: {
    alignItems: 'center',
    marginBottom: Spacing[6],
  },
  icon: {
    fontSize: 48,
    marginBottom: Spacing[3],
  },
  title: {
    fontSize: 20,
    fontWeight: '700',
    color: Colors.text,
    marginBottom: Spacing[2],
    textAlign: 'center',
  },
  subtitle: {
    fontSize: 14,
    color: Colors.text2,
    textAlign: 'center',
    lineHeight: 20,
  },
  content: {
    backgroundColor: 'rgba(239, 68, 68, 0.05)',
    borderRadius: 12,
    padding: Spacing[4],
    marginBottom: Spacing[6],
    borderLeftWidth: 4,
    borderLeftColor: '#ef4444',
  },
  contentText: {
    fontSize: 14,
    color: Colors.text,
    lineHeight: 20,
    marginBottom: Spacing[2],
  },
  stats: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    backgroundColor: Colors.border,
    borderRadius: 8,
    paddingVertical: Spacing[3],
    marginBottom: Spacing[6],
  },
  stat: {
    alignItems: 'center',
  },
  statValue: {
    fontSize: 18,
    fontWeight: '700',
    color: '#ef4444',
    marginBottom: Spacing[1],
  },
  statLabel: {
    fontSize: 12,
    color: Colors.text2,
  },
  actions: {
    gap: Spacing[3],
  },
  breakButton: {
    backgroundColor: '#fbbf24',
  },
  continueButton: {
    backgroundColor: Colors.border,
  },
  continueButtonText: {
    color: Colors.text,
  },
  partnershipLink: {
    fontSize: 12,
    color: '#3b82f6',
    textAlign: 'center',
    marginTop: Spacing[4],
    textDecorationLine: 'underline',
  },
});

interface LossStreakAlertProps {
  onTakeBreak?: () => void;
  onDismiss?: () => void;
}

export const LossStreakAlert: React.FC<LossStreakAlertProps> = ({
  onTakeBreak,
  onDismiss,
}) => {
  const lossStreakAlert = useRGStore((state) => state.lossStreakAlert);
  const showLossAlert = useRGStore((state) => state.showLossAlert);
  const dismissLossAlert = useRGStore((state) => state.dismissLossAlert);

  const handleTakeBreak = () => {
    dismissLossAlert();
    onTakeBreak?.();
  };

  const handleDismiss = () => {
    dismissLossAlert();
    onDismiss?.();
  };

  if (!showLossAlert || !lossStreakAlert) return null;

  const consecutiveLosses = lossStreakAlert.consecutiveLosses;
  const lossAmount = (lossStreakAlert.totalLossMinor / 1000000).toFixed(2);

  return (
    <Modal
      visible={showLossAlert}
      transparent
      animationType="slide"
      onRequestClose={handleDismiss}
    >
      <Pressable style={styles.modal} onPress={handleDismiss}>
        <Pressable style={styles.container} onPress={(e) => e.stopPropagation()}>
          <ScrollView showsVerticalScrollIndicator={false}>
            <View style={styles.header}>
              <Text style={styles.icon}>📊</Text>
              <Text style={styles.title}>Loss Streak Alert</Text>
              <Text style={styles.subtitle}>
                You've had {consecutiveLosses} consecutive losses. Consider taking a break.
              </Text>
            </View>

            <View style={styles.content}>
              <Text style={styles.contentText}>
                💡 <Text style={{ fontWeight: '600' }}>Take a moment</Text> to reflect on your
                betting strategy.
              </Text>
              <Text style={styles.contentText}>
                🎯 <Text style={{ fontWeight: '600' }}>Set limits</Text> to protect your bankroll.
              </Text>
              <Text style={styles.contentText}>
                ✋ <Text style={{ fontWeight: '600' }}>Take a cool-off</Text> if you need a break
                from betting.
              </Text>
            </View>

            <View style={styles.stats}>
              <View style={styles.stat}>
                <Text style={styles.statValue}>{consecutiveLosses}</Text>
                <Text style={styles.statLabel}>Consecutive</Text>
              </View>
              <View style={styles.stat}>
                <Text style={styles.statValue}>${lossAmount}</Text>
                <Text style={styles.statLabel}>Total Loss</Text>
              </View>
              <View style={styles.stat}>
                <Text style={styles.statValue}>{lossStreakAlert.marketIds.length}</Text>
                <Text style={styles.statLabel}>Markets</Text>
              </View>
            </View>

            <View style={styles.actions}>
              <Button
                title="Take a Cool-off Break"
                onPress={handleTakeBreak}
                style={styles.breakButton}
                textColor="#78350f"
              />
              <Button
                title="Continue Betting"
                onPress={handleDismiss}
                style={styles.continueButton}
                textColor={Colors.text}
              />
            </View>

            <Text style={styles.partnershipLink}>
              Need help? Contact Gambler's Anonymous →
            </Text>
          </ScrollView>
        </Pressable>
      </Pressable>
    </Modal>
  );
};
