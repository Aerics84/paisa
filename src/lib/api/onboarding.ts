import { request } from "./client";

export type OnboardingProfileId = "germany-eu" | "india";
export type OnboardingMode = "empty" | "demo";

export interface OnboardingProfile {
  id: OnboardingProfileId;
  label: string;
  currency: string;
  locale: string;
}

export interface OnboardingStatus {
  needed: boolean;
  can_setup: boolean;
  profiles: OnboardingProfile[];
}

export interface OnboardingResult {
  success: boolean;
  profile: OnboardingProfileId;
  mode: OnboardingMode;
}

// getOnboardingStatus probes the optional first-run endpoint without going
// through the redirecting client path. A 401/403 means onboarding is not
// available for this session, not that the user must be logged out.
export async function getOnboardingStatus(): Promise<OnboardingStatus> {
  const response = await fetch("/api/onboarding", {
    headers: { Accept: "application/json" },
    credentials: "same-origin"
  });

  if (response.status === 401 || response.status === 403) {
    return { needed: false, can_setup: false, profiles: [] };
  }

  if (!response.ok) {
    throw new Error(`Onboarding status failed with status ${response.status}`);
  }

  return (await response.json()) as OnboardingStatus;
}

export function runOnboarding(
  profile: OnboardingProfileId,
  mode: OnboardingMode
): Promise<OnboardingResult> {
  return request<OnboardingResult>("/api/onboarding", {
    method: "POST",
    body: JSON.stringify({ profile, mode })
  });
}
