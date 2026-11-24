import type { Dayjs } from "dayjs";
import { parseDate, request } from "./client";
import type { Point as WirePoint } from "./accounting.gen";
import type {
  RetirementGoalDetail as WireRetirementGoalDetail,
  SavingsGoalDetail as WireSavingsGoalDetail,
  GoalSummary as WireGoalSummary
} from "./goal.gen";
import { postingsFromWire, type Posting } from "./ledger";
import type { GoalContribution, GoalIntelligence as WireGoalIntelligence } from "./service.gen";
import type { GoalsResponse } from "./types.gen";

/**
 * Goal analysis with the literal unions the UI relies on. The backend values
 * are strings, so the generated type is narrowed here instead of at every
 * caller.
 */
export type GoalIntelligenceData = Omit<
  WireGoalIntelligence,
  "projectionSource" | "state" | "scheduleStatus" | "attention" | "deadlineSource"
> & {
  projectionSource?: "recent-pace" | "configured-payment" | "assumed-growth";
  state: "active" | "completed" | "overdue";
  scheduleStatus: "on-track" | "behind" | "at-risk" | "insufficient-data" | "no-deadline";
  attention: "none" | "warning" | "critical";
  deadlineSource: "configured" | "derived-from-payment" | "none";
};

/** Goal summary as rendered by the dashboard and goals list. */
export type GoalSummary = Omit<WireGoalSummary, "intelligence"> & {
  intelligence?: GoalIntelligenceData;
};

/** Goal contribution month; it has no dates, so the wire shape is reused. */
export type GoalContributionMonth = GoalContribution;

/** Savings timeline point with its date converted to dayjs. */
export type GoalPoint = Omit<WirePoint, "date"> & { date: Dayjs };

/** Retirement progress detail with the timeline and postings converted. */
export type RetirementGoalProgress = Omit<
  WireRetirementGoalDetail,
  "savingsTimeline" | "postings" | "intelligence"
> & {
  savingsTimeline: GoalPoint[];
  postings: Posting[];
  intelligence: GoalIntelligenceData;
};

/**
 * Savings progress detail with the timeline and postings converted. The
 * configured targetDate stays a YYYY-MM-DD string; only recorded timestamps
 * become Dayjs instances.
 */
export type SavingsGoalProgress = Omit<
  WireSavingsGoalDetail,
  "savingsTimeline" | "postings" | "intelligence"
> & {
  savingsTimeline: GoalPoint[];
  postings: Posting[];
  intelligence: GoalIntelligenceData;
};

function goalIntelligenceFromWire(wire: WireGoalIntelligence): GoalIntelligenceData {
  return wire as GoalIntelligenceData;
}

function goalPointFromWire(wire: WirePoint): GoalPoint {
  return { ...wire, date: parseDate(wire.date) };
}

/** Shared with the dashboard payload, which embeds the same summaries. */
export function goalSummaryFromWire(wire: WireGoalSummary): GoalSummary {
  return {
    ...wire,
    intelligence: wire.intelligence ? goalIntelligenceFromWire(wire.intelligence) : undefined
  };
}

function retirementGoalFromWire(wire: WireRetirementGoalDetail): RetirementGoalProgress {
  return {
    ...wire,
    savingsTimeline: (wire.savingsTimeline ?? []).map(goalPointFromWire),
    postings: postingsFromWire(wire.postings ?? []),
    balances: wire.balances ?? {},
    intelligence: goalIntelligenceFromWire(wire.intelligence)
  };
}

function savingsGoalFromWire(wire: WireSavingsGoalDetail): SavingsGoalProgress {
  return {
    ...wire,
    savingsTimeline: (wire.savingsTimeline ?? []).map(goalPointFromWire),
    postings: postingsFromWire(wire.postings ?? []),
    balances: wire.balances ?? {},
    contributionHistory: wire.contributionHistory ?? [],
    intelligence: goalIntelligenceFromWire(wire.intelligence)
  };
}

export async function getGoals(): Promise<{ goals: GoalSummary[] }> {
  const wire = await request<GoalsResponse>("/api/goals");
  return { goals: (wire.goals ?? []).map(goalSummaryFromWire) };
}

export async function getRetirementGoal(name: string): Promise<RetirementGoalProgress> {
  const wire = await request<WireRetirementGoalDetail>("/api/goals/retirement/:name", undefined, {
    name
  });
  return retirementGoalFromWire(wire);
}

export async function getSavingsGoal(name: string): Promise<SavingsGoalProgress> {
  const wire = await request<WireSavingsGoalDetail>("/api/goals/savings/:name", undefined, {
    name
  });
  return savingsGoalFromWire(wire);
}
