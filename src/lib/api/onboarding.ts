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
  profiles: OnboardingProfile[];
}

export interface OnboardingResult {
  success: boolean;
  profile: OnboardingProfileId;
  mode: OnboardingMode;
}

export function getOnboardingStatus(): Promise<OnboardingStatus> {
  return request<OnboardingStatus>("/api/onboarding");
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
