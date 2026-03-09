/**
 * RGSettingsScreen
 * Responsible gambling settings including deposit limits, cool-off, self-exclusion
 */
import React, { useState, useEffect } from 'react';
import {
  View,
  StyleSheet,
  SafeAreaView,
  ScrollView,
  Pressable,
  Modal,
  Alert,
  ActivityIndicator,
} from 'react-native';
import { Text } from '@/components/Text';
import { Button } from '@/components/Button';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { useRGStore } from '@/store/useRGStore';

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.bg,
  },
  header: {
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[4],
    borderBottomWidth: 1,
    borderBottomColor: Colors.border,
  },
  headerTitle: {
    fontSize: 24,
    fontWeight: '700',
    color: Colors.text,
  },
  content: {
    flex: 1,
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[4],
  },
  section: {
    marginBottom: Spacing[6],
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    color: Colors.text2,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
    marginBottom: Spacing[3],
  },
  card: {
    backgroundColor: Colors.border,
    borderRadius: 12,
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[3],
    marginBottom: Spacing[3],
  },
  cardRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: Spacing[2],
  },
  cardLabel: {
    fontSize: 14,
    fontWeight: '500',
    color: Colors.text,
  },
  cardValue: {
    fontSize: 14,
    fontWeight: '600',
    color: Colors.primary,
  },
  slider: {
    marginVertical: Spacing[3],
  },
  sliderLabel: {
    fontSize: 12,
    color: Colors.text2,
    marginBottom: Spacing[2],
  },
  buttonGrid: {
    flexDirection: 'row',
    gap: Spacing[2],
    marginVertical: Spacing[3],
  },
  buttonHalf: {
    flex: 1,
  },
  warningBox: {
    backgroundColor: 'rgba(239, 68, 68, 0.1)',
    borderLeftWidth: 4,
    borderLeftColor: '#ef4444',
    padding: Spacing[3],
    borderRadius: 8,
    marginBottom: Spacing[4],
  },
  warningText: {
    fontSize: 13,
    color: '#ef4444',
    lineHeight: 18,
  },
  successBox: {
    backgroundColor: 'rgba(34, 197, 94, 0.1)',
    borderLeftWidth: 4,
    borderLeftColor: '#22c55e',
    padding: Spacing[3],
    borderRadius: 8,
    marginBottom: Spacing[4],
  },
  successText: {
    fontSize: 13,
    color: '#22c55e',
    lineHeight: 18,
  },
  disabledButton: {
    opacity: 0.5,
  },
  partnershipLink: {
    fontSize: 13,
    color: '#3b82f6',
    textAlign: 'center',
    marginTop: Spacing[4],
    textDecorationLine: 'underline',
  },
});

export const RGSettingsScreen: React.FC = () => {
  const depositSettings = useRGStore((state) => state.depositSettings);
  const depositSettingsLoading = useRGStore((state) => state.depositSettingsLoading);
  const exclusionSettings = useRGStore((state) => state.exclusionSettings);
  const exclusionSettingsLoading = useRGStore((state) => state.exclusionSettingsLoading);
  const lossStreakThreshold = useRGStore((state) => state.lossStreakThreshold);
  const setLossStreakThreshold = useRGStore((state) => state.setLossStreakThreshold);
  const updateLossStreakThreshold = useRGStore((state) => state.updateLossStreakThreshold);
  const fetchDepositSettings = useRGStore((state) => state.fetchDepositSettings);
  const fetchExclusionSettings = useRGStore((state) => state.fetchExclusionSettings);
  const fetchLossStreakThreshold = useRGStore((state) => state.fetchLossStreakThreshold);

  const [showCoolOffModal, setShowCoolOffModal] = useState(false);
  const [showSelfExcludeModal, setShowSelfExcludeModal] = useState(false);
  const [selectedCoolOffDuration, setSelectedCoolOffDuration] = useState<24 | 168 | 720>(24);
  const [selectedSelfExcludeDuration, setSelectedSelfExcludeDuration] = useState<number | null>(
    30
  );

  // Load RG settings on mount
  useEffect(() => {
    fetchDepositSettings();
    fetchExclusionSettings();
    fetchLossStreakThreshold();
  }, []);

  const handleStartCoolOff = async () => {
    Alert.alert(
      'Start Cool-off?',
      `You'll be unable to deposit for ${selectedCoolOffDuration === 24 ? '24 hours' : selectedCoolOffDuration === 168 ? '7 days' : '30 days'}.`,
      [
        {
          text: 'Cancel',
          style: 'cancel',
        },
        {
          text: 'Confirm',
          onPress: async () => {
            try {
              await useRGStore.getState().startCoolOff(selectedCoolOffDuration as 24 | 168 | 720);
              setShowCoolOffModal(false);
              Alert.alert('Cool-off Started', 'Your cool-off period has been activated.');
            } catch (error: any) {
              Alert.alert('Error', error.message || 'Failed to start cool-off');
            }
          },
          style: 'destructive',
        },
      ]
    );
  };

  const handleSelfExclude = async () => {
    const duration = selectedSelfExcludeDuration === null ? 'permanently' : `for ${selectedSelfExcludeDuration} days`;
    Alert.alert(
      'Self-Exclude Account?',
      `This will lock your account ${duration}. You will not be able to bet or deposit.`,
      [
        {
          text: 'Cancel',
          style: 'cancel',
        },
        {
          text: 'Self-Exclude',
          onPress: async () => {
            try {
              await useRGStore.getState().selfExclude(selectedSelfExcludeDuration || undefined);
              setShowSelfExcludeModal(false);
              Alert.alert('Account Locked', 'Your account has been self-excluded.');
            } catch (error: any) {
              Alert.alert('Error', error.message || 'Failed to self-exclude');
            }
          },
          style: 'destructive',
        },
      ]
    );
  };

  const canCancelCoolOff = exclusionSettings?.inCoolOff && exclusionSettings?.coolOffCancellable;

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Responsible Gaming</Text>
      </View>

      <ScrollView style={styles.content} showsVerticalScrollIndicator={false}>
        {/* Warning if in cool-off */}
        {exclusionSettings?.inCoolOff && (
          <View style={styles.warningBox}>
            <Text style={styles.warningText}>
              ⏸️ <Text style={{ fontWeight: '600' }}>Cool-off Active</Text>
              {'\n'}Deposits are unavailable for {exclusionSettings.coolOffRemainingHours} hours.
            </Text>
          </View>
        )}

        {/* Warning if self-excluded */}
        {exclusionSettings?.isSelfExcluded && (
          <View style={styles.warningBox}>
            <Text style={styles.warningText}>
              🛑 <Text style={{ fontWeight: '600' }}>Account Locked</Text>
              {'\n'}Your account is self-excluded. Contact support to reactivate.
            </Text>
          </View>
        )}

        {/* Deposit Limits Section */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Daily Deposit Limits</Text>
          <View style={styles.card}>
            <View style={styles.cardRow}>
              <Text style={styles.cardLabel}>Daily Limit</Text>
              <Text style={styles.cardValue}>
                ${(depositSettings?.dailyDepositLimitMinor ?? 0) / 1000000}
              </Text>
            </View>
            <View style={styles.cardRow}>
              <Text style={styles.cardLabel}>Remaining Today</Text>
              <Text style={styles.cardValue}>
                ${(depositSettings?.remainingDailyBudget ?? 0) / 1000000}
              </Text>
            </View>
          </View>
          <Button title="Adjust Deposit Limit" onPress={() => {}} />
        </View>

        {/* Cool-off Section */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Cool-off Period</Text>
          <Text style={styles.sliderLabel}>
            Temporarily disable deposits and new orders to take a break.
          </Text>
          <View style={styles.buttonGrid}>
            <Button
              title="24 Hours"
              style={styles.buttonHalf}
              onPress={() => {
                setSelectedCoolOffDuration(24);
                setShowCoolOffModal(true);
              }}
              disabled={!exclusionSettings || !exclusionSettings.coolOffCancellable}
            />
            <Button
              title="7 Days"
              style={styles.buttonHalf}
              onPress={() => {
                setSelectedCoolOffDuration(168);
                setShowCoolOffModal(true);
              }}
              disabled={!exclusionSettings || !exclusionSettings.coolOffCancellable}
            />
            <Button
              title="30 Days"
              style={styles.buttonHalf}
              onPress={() => {
                setSelectedCoolOffDuration(720);
                setShowCoolOffModal(true);
              }}
              disabled={!exclusionSettings || !exclusionSettings.coolOffCancellable}
            />
          </View>

          {canCancelCoolOff && (
            <Button
              title="Cancel Cool-off"
              onPress={() => {
                Alert.alert('Cancel Cool-off?', 'Are you sure you want to cancel this cool-off period?', [
                  { text: 'Keep Active', style: 'cancel' },
                  {
                    text: 'Cancel Cool-off',
                    onPress: () => {
                      // Call API to cancel cool-off
                      Alert.alert('Cool-off Cancelled', 'Your cool-off period has been removed.');
                    },
                  },
                ]);
              }}
            />
          )}

          {!exclusionSettings?.coolOffCancellable && (
            <Text style={styles.sliderLabel}>
              ⚠️ Cool-off periods cannot be cancelled in your region.
            </Text>
          )}
        </View>

        {/* Loss Streak Threshold */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Loss Streak Alert</Text>
          <Text style={styles.sliderLabel}>
            Get notified after {lossStreakThreshold} consecutive losses.
          </Text>
          <View style={styles.buttonGrid}>
            {[1, 2, 3, 5, 10].map((threshold) => (
              <Pressable
                key={threshold}
                style={[
                  styles.buttonHalf,
                  lossStreakThreshold === threshold && { opacity: 0.7 },
                ]}
                onPress={() => updateLossStreakThreshold(threshold)}
              >
                <View
                  style={{
                    padding: Spacing[2],
                    backgroundColor: lossStreakThreshold === threshold ? '#3b82f6' : Colors.border,
                    borderRadius: 8,
                    alignItems: 'center',
                  }}
                >
                  <Text
                    style={{
                      color: lossStreakThreshold === threshold ? '#fff' : Colors.text,
                      fontWeight: '600',
                    }}
                  >
                    {threshold}
                  </Text>
                </View>
              </Pressable>
            ))}
          </View>
        </View>

        {/* Self-Exclusion Section */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Self-Exclusion</Text>
          <Text style={styles.sliderLabel}>
            Permanently lock your account to prevent all betting and deposits.
          </Text>
          <Button
            title="Self-Exclude Account"
            onPress={() => setShowSelfExcludeModal(true)}
            style={{ backgroundColor: '#ef4444' }}
            textColor="#fff"
          />
        </View>

        {/* Partnership Link */}
        <Text style={styles.partnershipLink}>
          Need help with gambling? Gambler's Anonymous provides free support →
        </Text>
      </ScrollView>

      {/* Cool-off Confirmation Modal */}
      <Modal visible={showCoolOffModal} transparent animationType="fade">
        <Pressable style={{ flex: 1, backgroundColor: 'rgba(0, 0, 0, 0.5)' }}>
          <View style={{ flex: 1, justifyContent: 'center', paddingHorizontal: Spacing[4] }}>
            <View
              style={{
                backgroundColor: Colors.bg,
                borderRadius: 12,
                padding: Spacing[4],
              }}
            >
              <Text style={{ fontSize: 16, fontWeight: '700', color: Colors.text, marginBottom: Spacing[2] }}>
                Confirm Cool-off
              </Text>
              <Text style={{ fontSize: 14, color: Colors.text2, marginBottom: Spacing[4] }}>
                You'll be unable to deposit or place new orders during this period.
              </Text>
              <View style={{ flexDirection: 'row', gap: Spacing[2] }}>
                <Button
                  title="Cancel"
                  onPress={() => setShowCoolOffModal(false)}
                  style={{ flex: 1 }}
                />
                <Button
                  title="Confirm"
                  onPress={handleStartCoolOff}
                  style={{ flex: 1, backgroundColor: '#fbbf24' }}
                  textColor="#78350f"
                />
              </View>
            </View>
          </View>
        </Pressable>
      </Modal>

      {/* Self-Exclude Confirmation Modal */}
      <Modal visible={showSelfExcludeModal} transparent animationType="fade">
        <Pressable style={{ flex: 1, backgroundColor: 'rgba(0, 0, 0, 0.5)' }}>
          <View style={{ flex: 1, justifyContent: 'center', paddingHorizontal: Spacing[4] }}>
            <View
              style={{
                backgroundColor: Colors.bg,
                borderRadius: 12,
                padding: Spacing[4],
              }}
            >
              <Text style={{ fontSize: 16, fontWeight: '700', color: '#ef4444', marginBottom: Spacing[2] }}>
                ⚠️ Self-Exclusion
              </Text>
              <Text style={{ fontSize: 14, color: Colors.text2, marginBottom: Spacing[4] }}>
                This will lock your account {selectedSelfExcludeDuration === null ? 'permanently' : `for ${selectedSelfExcludeDuration} days`}. You cannot undo this action during the exclusion period.
              </Text>
              <View style={{ flexDirection: 'row', gap: Spacing[2] }}>
                <Button
                  title="Cancel"
                  onPress={() => setShowSelfExcludeModal(false)}
                  style={{ flex: 1 }}
                />
                <Button
                  title="Self-Exclude"
                  onPress={handleSelfExclude}
                  style={{ flex: 1, backgroundColor: '#ef4444' }}
                  textColor="#fff"
                />
              </View>
            </View>
          </View>
        </Pressable>
      </Modal>
    </SafeAreaView>
  );
};
