/**
 * Auth Store Tests
 */
import { renderHook, act } from '@testing-library/react-native';
import { useAuthStore } from '../useAuthStore';

describe('useAuthStore', () => {
  beforeEach(() => {
    useAuthStore.setState({
      user: null,
      accessToken: null,
      refreshToken: null,
      onboardingStep: 0,
      isAuthenticated: false,
    });
  });

  it('sets user correctly', () => {
    const { result } = renderHook(() => useAuthStore());

    act(() => {
      result.current.setUser({
        id: '1',
        phoneNumber: '+911234567890',
        displayName: 'John Doe',
        walletId: 'wallet-1',
        createdAt: new Date().toISOString(),
      });
    });

    expect(result.current.user?.displayName).toBe('John Doe');
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('sets tokens correctly', () => {
    const { result } = renderHook(() => useAuthStore());

    act(() => {
      result.current.setTokens('access-token', 'refresh-token');
    });

    expect(result.current.accessToken).toBe('access-token');
    expect(result.current.refreshToken).toBe('refresh-token');
  });

  it('sets onboarding step', () => {
    const { result } = renderHook(() => useAuthStore());

    act(() => {
      result.current.setOnboardingStep(3);
    });

    expect(result.current.onboardingStep).toBe(3);
  });

  it('logs out correctly', () => {
    const { result } = renderHook(() => useAuthStore());

    act(() => {
      result.current.setUser({
        id: '1',
        phoneNumber: '+911234567890',
        displayName: 'John Doe',
        walletId: 'wallet-1',
        createdAt: new Date().toISOString(),
      });
      result.current.setTokens('access-token', 'refresh-token');
      result.current.setOnboardingStep(5);
    });

    act(() => {
      result.current.logout();
    });

    expect(result.current.user).toBeNull();
    expect(result.current.accessToken).toBeNull();
    expect(result.current.refreshToken).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    expect(result.current.onboardingStep).toBe(0);
  });
});
