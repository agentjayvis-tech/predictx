# Onboarding Flow Design

## Overview

The PredictX onboarding flow is a **45-second guided tour** designed to get users from app install to their first prediction in under 1 minute.

## User Journey

### Pre-Onboarding
1. User downloads app from App Store/Play Store
2. App launches
3. Splash screen (2 sec) → RootNavigator checks `onboardingStep`
4. Redirect to OnboardingScreen if `onboardingStep < 5`

### Onboarding Screens (45 seconds total)

#### Step 1: Welcome (8 seconds)
- **Icon**: 🎯
- **Title**: "Welcome to PredictX"
- **Description**: "Predict outcomes. Earn rewards. Join the prediction revolution."
- **Auto-advance**: Yes, after 8 seconds
- **Action**: Next / Skip Tour

#### Step 2: How It Works (8 seconds)
- **Icon**: 📱
- **Title**: "How It Works"
- **Description**: "Swipe to bet on outcomes. YES or NO. Simple. Fast. Fair."
- **Auto-advance**: Yes, after 8 seconds
- **Demo**: Show swipe gesture animation
- **Action**: Next / Skip Tour

#### Step 3: Earn & Settle (8 seconds)
- **Icon**: 💰
- **Title**: "Earn & Settle"
- **Description**: "Correct predictions = instant payouts. No delays."
- **Auto-advance**: Yes, after 8 seconds
- **Highlight**: Fast settlement process
- **Action**: Next / Skip Tour

#### Step 4: Free Play (8 seconds)
- **Icon**: 🎁
- **Title**: "Free Play"
- **Description**: "Start with ₹100 free play credits. No payment required."
- **Auto-advance**: Yes, after 8 seconds
- **CTA**: Get ₹100 free credits
- **Action**: Next / Skip Tour

#### Step 5: Ready to Predict (13 seconds)
- **Icon**: 🚀
- **Title**: "Ready to Predict?"
- **Description**: "Tap below to enable notifications and get started."
- **Auto-advance**: No (requires action)
- **Primary Action**: "Get Started" → Enable notifications → Main feed
- **Secondary Action**: "Skip Tour"

### Total Duration
```
Step 1: 8 sec
Step 2: 8 sec
Step 3: 8 sec
Step 4: 8 sec
Step 5: 13 sec (requires user action)
---
Total: 45 seconds (auto-advance only)
      60-90 seconds (with user reading)
```

## Implementation Details

### Component: OnboardingScreen

```tsx
// Auto-advance logic
useEffect(() => {
  const timer = setInterval(() => {
    if (timeLeft <= 1) {
      if (currentStep < STEPS.length - 1) {
        nextStep();  // Auto-advance
      } else {
        completeOnboarding();  // Finish
      }
    }
    setTimeLeft(prev => prev - 1);
  }, 1000);

  return () => clearInterval(timer);
}, [currentStep]);

// User can skip or force next
const skipOnboarding = () => completeOnboarding();
const nextStep = () => { /* advance */ };
```

### State Management

```typescript
// useAuthStore
{
  onboardingStep: 0-5,
  // 0: Not started
  // 1-4: In progress
  // 5: Complete
}

// On completion
setOnboardingStep(5);
// Triggers RootNavigator switch to MainTabs
```

### Visual Design

**Colors**
- Background: #080810 (dark bg)
- Icon: Large emoji (64pt)
- Title: h2 variant (26pt, bold)
- Description: body variant (15pt, text2 color)

**Animation**
- Fade transition between steps
- Progress bar (animated width)
- Button press feedback

**Responsive**
- Works on 4.5" to 6.7" screens
- Safe area insets for notch devices
- Portrait mode only

## Skip Tour Behavior

If user taps "Skip Tour":
1. `setOnboardingStep(5)`
2. Navigate to `MainTabs`
3. Onboarding screen unmounts
4. **Note**: Can't go back to onboarding (future: settings to re-enable)

## Analytics Events

Track onboarding completion:

```typescript
// On step complete
analytics.logEvent('onboarding_step_complete', {
  step: currentStep,
  duration: STEPS[currentStep].duration,
  skipped: false,
});

// On skip
analytics.logEvent('onboarding_skipped', {
  step: currentStep,
  reason: 'user_skip',
});

// On completion
analytics.logEvent('onboarding_complete', {
  totalDuration: totalSeconds,
  method: 'auto_advance' | 'skip',
});
```

## Testing Checklist

- [ ] All 5 steps render correctly
- [ ] Auto-advance works (each step advances)
- [ ] Skip button skips to main feed
- [ ] Progress bar fills smoothly
- [ ] Timer counts down accurately
- [ ] Responsive on 4.5"+ screens
- [ ] Safe area respected (notch devices)
- [ ] Tap any step to go back (future)
- [ ] No memory leaks (timer cleanup)
- [ ] Emoji render on iOS & Android

## Future Enhancements

1. **Personalization**
   - Skip onboarding for returning users
   - Show app tour based on user segment

2. **Interactive**
   - Tap to pause/resume countdown
   - Drag progress bar to previous step
   - Swipe left/right to navigate

3. **Analytics**
   - Track completion rate by step
   - A/B test different messaging
   - Identify drop-off points

4. **Localization**
   - Multi-language support
   - Regional customization (₹ vs other currencies)
   - Time-based content (e.g., "earn during cricket season")

## Related Files

- `/src/screens/OnboardingScreen.tsx` — Main component
- `/src/store/useAuthStore.ts` — State management
- `/src/navigation/RootNavigator.tsx` — Navigation logic
- `/src/theme/` — Design tokens
