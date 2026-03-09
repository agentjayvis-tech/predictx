/**
 * OnboardingScreen
 * 45-second onboarding flow for new users
 */
import React, { useState, useEffect } from 'react';
import {
  View,
  StyleSheet,
  ScrollView,
  SafeAreaView,
} from 'react-native';
import { Text } from '@/components/Text';
import { Button } from '@/components/Button';
import { Colors } from '@/theme/colors';
import { Spacing } from '@/theme/spacing';
import { useAuthStore } from '@/store/useAuthStore';

interface OnboardingStep {
  id: number;
  title: string;
  description: string;
  icon: string;
  duration: number; // seconds
}

const ONBOARDING_STEPS: OnboardingStep[] = [
  {
    id: 1,
    title: 'Welcome to PredictX',
    description: 'Predict outcomes. Earn rewards. Join the prediction revolution.',
    icon: '🎯',
    duration: 8,
  },
  {
    id: 2,
    title: 'How It Works',
    description: 'Swipe to bet on outcomes. YES or NO. Simple. Fast. Fair.',
    icon: '📱',
    duration: 8,
  },
  {
    id: 3,
    title: 'Earn & Settle',
    description: 'Correct predictions = instant payouts. No delays.',
    icon: '💰',
    duration: 8,
  },
  {
    id: 4,
    title: 'Free Play',
    description: 'Start with ₹100 free play credits. No payment required.',
    icon: '🎁',
    duration: 8,
  },
  {
    id: 5,
    title: 'Ready to Predict?',
    description: 'Tap below to enable notifications and get started.',
    icon: '🚀',
    duration: 13,
  },
];

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.bg,
  },
  content: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: Spacing[4],
  },
  icon: {
    fontSize: 64,
    marginBottom: Spacing[6],
    textAlign: 'center',
  },
  title: {
    textAlign: 'center',
    marginBottom: Spacing[3],
  },
  description: {
    textAlign: 'center',
    marginBottom: Spacing[8],
    color: Colors.text2,
    lineHeight: 22,
  },
  progressBar: {
    height: 4,
    backgroundColor: Colors.surface2,
    borderRadius: 2,
    marginBottom: Spacing[6],
    overflow: 'hidden',
  },
  progressFill: {
    height: '100%',
    backgroundColor: Colors.accent,
  },
  buttons: {
    gap: Spacing[3],
  },
  skipButton: {
    marginBottom: Spacing[2],
  },
  timer: {
    textAlign: 'center',
    color: Colors.text2,
    marginBottom: Spacing[4],
    fontSize: 12,
  },
});

export const OnboardingScreen: React.FC = () => {
  const [currentStep, setCurrentStep] = useState(0);
  const [timeLeft, setTimeLeft] = useState(ONBOARDING_STEPS[0].duration);
  const { setOnboardingStep } = useAuthStore();

  useEffect(() => {
    const timer = setInterval(() => {
      setTimeLeft((prev) => {
        if (prev <= 1) {
          // Move to next step or complete onboarding
          if (currentStep < ONBOARDING_STEPS.length - 1) {
            setCurrentStep((s) => s + 1);
            return ONBOARDING_STEPS[currentStep + 1].duration;
          } else {
            completeOnboarding();
            return 0;
          }
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [currentStep]);

  const completeOnboarding = () => {
    setOnboardingStep(5);
  };

  const skipOnboarding = () => {
    completeOnboarding();
  };

  const nextStep = () => {
    if (currentStep < ONBOARDING_STEPS.length - 1) {
      setCurrentStep((s) => s + 1);
      setTimeLeft(ONBOARDING_STEPS[currentStep + 1].duration);
    } else {
      completeOnboarding();
    }
  };

  const step = ONBOARDING_STEPS[currentStep];
  const progress = ((currentStep + 1) / ONBOARDING_STEPS.length) * 100;
  const totalSeconds = ONBOARDING_STEPS.reduce((sum, s) => sum + s.duration, 0);

  return (
    <SafeAreaView style={styles.container}>
      <ScrollView contentContainerStyle={{ flexGrow: 1 }}>
        <View style={styles.progressBar}>
          <View
            style={[
              styles.progressFill,
              { width: `${progress}%` },
            ]}
          />
        </View>

        <View style={styles.content}>
          <Text style={styles.icon}>{step.icon}</Text>
          <Text variant="h2" style={styles.title}>
            {step.title}
          </Text>
          <Text style={styles.description}>
            {step.description}
          </Text>

          <Text style={styles.timer}>
            Auto-advance in {timeLeft}s ({currentStep + 1}/{ONBOARDING_STEPS.length})
          </Text>

          <View style={styles.buttons}>
            <Button
              label={currentStep === ONBOARDING_STEPS.length - 1 ? 'Get Started' : 'Next'}
              onPress={nextStep}
            />
            <Button
              label="Skip Tour"
              variant="secondary"
              onPress={skipOnboarding}
              style={styles.skipButton}
            />
          </View>
        </View>
      </ScrollView>
    </SafeAreaView>
  );
};
