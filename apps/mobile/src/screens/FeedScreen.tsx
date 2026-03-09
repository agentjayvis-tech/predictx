/**
 * FeedScreen
 * Main feed of markets with swipe-to-bet functionality
 */
import React, { useState, useEffect } from 'react';
import {
  View,
  StyleSheet,
  SafeAreaView,
  FlatList,
  ActivityIndicator,
  RefreshControl,
} from 'react-native';
import { Text } from '@/components/Text';
import { MarketCard } from '@/components/MarketCard';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { useMarketStore, Market } from '@/store/useMarketStore';

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.bg,
  },
  header: {
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[3],
    borderBottomWidth: 1,
    borderBottomColor: Colors.border,
  },
  headerTitle: {
    marginBottom: Spacing[1],
  },
  headerSubtitle: {
    color: Colors.text2,
    fontSize: 13,
  },
  content: {
    flex: 1,
    paddingHorizontal: Spacing[4],
    paddingVertical: Spacing[3],
  },
  loading: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  empty: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: Spacing[4],
  },
  emptyText: {
    color: Colors.text2,
    marginBottom: Spacing[3],
  },
});

export const FeedScreen: React.FC = () => {
  const [refreshing, setRefreshing] = useState(false);
  const [loading, setLoading] = useState(false);
  const { markets, setMarkets } = useMarketStore();

  useEffect(() => {
    loadMarkets();
  }, []);

  const loadMarkets = async () => {
    setLoading(true);
    try {
      // Mock data - replace with API call
      const mockMarkets: Market[] = [
        {
          id: '1',
          question: 'Will India win the next Cricket World Cup?',
          category: 'Sports',
          closesAt: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
          resolvesAt: new Date(Date.now() + 31 * 24 * 60 * 60 * 1000).toISOString(),
          outcomes: [
            { id: 'yes', title: 'Yes', yesOdds: 1.85, noOdds: 2.05, volume: 1000000 },
          ],
          volume: 1000000,
          createdAt: new Date().toISOString(),
          description: 'Prediction market for India winning the next Cricket World Cup',
        },
        {
          id: '2',
          question: 'Will Bitcoin reach $100K by end of 2026?',
          category: 'Finance',
          closesAt: new Date(Date.now() + 60 * 24 * 60 * 60 * 1000).toISOString(),
          resolvesAt: new Date(Date.now() + 65 * 24 * 60 * 60 * 1000).toISOString(),
          outcomes: [
            { id: 'yes', title: 'Yes', yesOdds: 1.95, noOdds: 1.95, volume: 500000 },
          ],
          volume: 500000,
          createdAt: new Date().toISOString(),
        },
        {
          id: '3',
          question: 'Will it rain tomorrow in Mumbai?',
          category: 'Weather',
          closesAt: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
          resolvesAt: new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString(),
          outcomes: [
            { id: 'yes', title: 'Yes', yesOdds: 1.5, noOdds: 2.5, volume: 250000 },
          ],
          volume: 250000,
          createdAt: new Date().toISOString(),
        },
      ];
      setMarkets(mockMarkets);
    } catch (error) {
      console.error('Failed to load markets:', error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const handleRefresh = () => {
    setRefreshing(true);
    loadMarkets();
  };

  const handleSwipeLeft = (marketId: string) => {
    console.log('Swipe left (skip):', marketId);
  };

  const handleSwipeRight = (marketId: string) => {
    console.log('Swipe right (bet):', marketId);
  };

  if (loading && markets.length === 0) {
    return (
      <View style={[styles.container, styles.loading]}>
        <ActivityIndicator size="large" color={Colors.accent} />
      </View>
    );
  }

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text variant="h2" style={styles.headerTitle}>
          Predict
        </Text>
        <Text style={styles.headerSubtitle}>
          {markets.length} markets • Swipe to bet
        </Text>
      </View>

      <FlatList
        data={markets}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <MarketCard
            market={item}
            onSwipeLeft={() => handleSwipeLeft(item.id)}
            onSwipeRight={() => handleSwipeRight(item.id)}
            onPress={() => console.log('Market tapped:', item.id)}
          />
        )}
        contentContainerStyle={styles.content}
        refreshControl={
          <RefreshControl
            refreshing={refreshing}
            onRefresh={handleRefresh}
            tintColor={Colors.accent}
          />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>No markets available</Text>
          </View>
        }
      />
    </SafeAreaView>
  );
};
