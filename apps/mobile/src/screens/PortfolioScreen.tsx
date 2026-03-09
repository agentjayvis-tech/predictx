/**
 * PortfolioScreen
 * User portfolio and balance information
 */
import React, { useEffect } from 'react';
import {
  View,
  StyleSheet,
  SafeAreaView,
  ScrollView,
} from 'react-native';
import { Text } from '@/components/Text';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { useWalletStore } from '@/store/useWalletStore';
import { useAuthStore } from '@/store/useAuthStore';

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
  balanceCard: {
    backgroundColor: Colors.surface2,
    borderRadius: 16,
    padding: Spacing[4],
    marginBottom: Spacing[4],
    borderWidth: 1,
    borderColor: Colors.border,
  },
  balanceLabel: {
    color: Colors.text2,
    marginBottom: Spacing[2],
  },
  balanceAmount: {
    fontSize: 36,
    color: Colors.accent,
    marginBottom: Spacing[2],
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
    marginBottom: Spacing[3],
  },
  emptyState: {
    textAlign: 'center',
    color: Colors.text2,
    marginTop: Spacing[6],
  },
  statRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: Spacing[2],
    borderBottomWidth: 1,
    borderBottomColor: Colors.surface3,
  },
  statLabel: {
    color: Colors.text2,
  },
  statValue: {
    color: Colors.accent,
  },
});

export const PortfolioScreen: React.FC = () => {
  const { balance, currency } = useWalletStore();
  const { user } = useAuthStore();

  const stats = [
    { label: 'Total Bets', value: '12' },
    { label: 'Win Rate', value: '67%' },
    { label: 'Total Winnings', value: '₹2,450' },
    { label: 'Active Positions', value: '3' },
  ];

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text variant="h2">Portfolio</Text>
        <Text color={Colors.text2}>Hi, {user?.displayName || 'User'}!</Text>
      </View>

      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.balanceCard}>
          <Text style={styles.balanceLabel}>Available Balance</Text>
          <Text style={styles.balanceAmount}>
            ₹{balance.toFixed(2)}
          </Text>
          <Text color={Colors.text2} size="sm">
            {currency} • Ready to predict
          </Text>
        </View>

        <View style={styles.section}>
          <Text variant="h3" style={styles.sectionTitle}>
            Your Stats
          </Text>
          {stats.map((stat, index) => (
            <View key={index} style={styles.statRow}>
              <Text style={styles.statLabel}>{stat.label}</Text>
              <Text style={styles.statValue} weight="semibold">
                {stat.value}
              </Text>
            </View>
          ))}
        </View>

        <View style={styles.section}>
          <Text variant="h3" style={styles.sectionTitle}>
            Recent Activity
          </Text>
          <Text style={styles.emptyState}>
            No recent transactions
          </Text>
        </View>
      </ScrollView>
    </SafeAreaView>
  );
};
