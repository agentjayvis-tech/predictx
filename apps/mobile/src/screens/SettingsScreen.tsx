/**
 * SettingsScreen
 * User settings and preferences
 */
import React from 'react';
import {
  View,
  StyleSheet,
  SafeAreaView,
  ScrollView,
  Switch,
} from 'react-native';
import { Text } from '@/components/Text';
import { Button } from '@/components/Button';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
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
    color: Colors.text2,
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  settingRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: Spacing[3],
    borderBottomWidth: 1,
    borderBottomColor: Colors.surface3,
  },
  settingLabel: {
    flex: 1,
  },
  logoutButton: {
    marginTop: Spacing[8],
  },
});

interface Setting {
  label: string;
  description?: string;
  value: boolean;
  onChange: (value: boolean) => void;
}

export const SettingsScreen: React.FC = () => {
  const { logout, user } = useAuthStore();
  const [settings, setSettings] = React.useState({
    notifications: true,
    marketAlerts: true,
    priceAlerts: true,
    darkMode: true,
  });

  const handleLogout = () => {
    logout();
  };

  const handleSettingChange = (key: keyof typeof settings) => {
    setSettings((prev) => ({
      ...prev,
      [key]: !prev[key],
    }));
  };

  const settingsList: Array<{ key: keyof typeof settings; label: string; description?: string }> = [
    { key: 'notifications', label: 'Push Notifications', description: 'Receive alerts about your bets' },
    { key: 'marketAlerts', label: 'Market Alerts', description: 'Know when markets are about to close' },
    { key: 'priceAlerts', label: 'Price Alerts', description: 'Alerts when odds change significantly' },
    { key: 'darkMode', label: 'Dark Mode', description: 'Reduces eye strain' },
  ];

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text variant="h2">Settings</Text>
        <Text color={Colors.text2}>{user?.phoneNumber}</Text>
      </View>

      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Notifications</Text>
          {settingsList.slice(0, 3).map((setting) => (
            <View key={setting.key} style={styles.settingRow}>
              <View style={styles.settingLabel}>
                <Text>{setting.label}</Text>
                {setting.description && (
                  <Text color={Colors.text2} size="sm">
                    {setting.description}
                  </Text>
                )}
              </View>
              <Switch
                value={settings[setting.key]}
                onValueChange={() => handleSettingChange(setting.key)}
                trackColor={{ false: Colors.surface3, true: Colors.accent }}
              />
            </View>
          ))}
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Display</Text>
          <View style={styles.settingRow}>
            <View style={styles.settingLabel}>
              <Text>Dark Mode</Text>
              <Text color={Colors.text2} size="sm">
                Reduces eye strain
              </Text>
            </View>
            <Switch
              value={settings.darkMode}
              onValueChange={() => handleSettingChange('darkMode')}
              trackColor={{ false: Colors.surface3, true: Colors.accent }}
            />
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Account</Text>
          <Button
            label="Logout"
            variant="tertiary"
            onPress={handleLogout}
            style={styles.logoutButton}
          />
        </View>
      </ScrollView>
    </SafeAreaView>
  );
};
