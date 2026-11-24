import { afterEach, expect, test, mock } from "bun:test";
import dayjs from "dayjs";
import { getGoals, getRetirementGoal, getSavingsGoal } from "./goals";

const originalFetch = globalThis.fetch;
afterEach(() => {
  globalThis.fetch = originalFetch;
});

function mockJSON(body: unknown) {
  globalThis.fetch = mock(
    async () => new Response(JSON.stringify(body))
  ) as unknown as typeof fetch;
}

const posting = {
  id: 1,
  transaction_id: "t1",
  date: "2026-10-31T00:00:00Z",
  payee: "Fixture Payee",
  account: "Assets:Goal:Retirement",
  commodity: "EUR",
  quantity: 20000,
  amount: 20000,
  status: "cleared",
  tag_recurring: "",
  tag_period: "",
  tax_category: "",
  tax_category_legacy: "",
  transaction_begin_line: 1,
  transaction_end_line: 2,
  file_name: "main.ledger",
  forecast: false,
  note: "",
  transaction_note: "",
  market_amount: 20000,
  balance: 20000
};

const intelligence = {
  progressRatio: 0.02,
  remainingAmount: 980000,
  validTarget: true,
  requiredMonthlyContribution: 100,
  actualMonthlyContribution: 0,
  contributionMonthsObserved: 1,
  activeContributionMonths: 1,
  state: "active",
  scheduleStatus: "no-deadline",
  attention: "none",
  deadlineSource: "none",
  reasons: [] as string[]
};

test("getGoals maps summaries and defaults a null goal list", async () => {
  mockJSON({
    goals: [
      {
        type: "savings",
        name: "Vacation",
        id: "savings-Vacation",
        icon: "vacation",
        current: 4000,
        target: 10000,
        targetDate: "2027-06-30",
        priority: 2,
        intelligence
      }
    ]
  });

  const response = await getGoals();
  expect(response.goals).toHaveLength(1);
  expect(response.goals[0].name).toBe("Vacation");
  expect(response.goals[0].targetDate).toBe("2027-06-30");
  expect(response.goals[0].intelligence?.scheduleStatus).toBe("no-deadline");

  mockJSON({ goals: null });
  await expect(getGoals()).resolves.toEqual({ goals: [] });
});

test("getRetirementGoal converts timeline and posting dates", async () => {
  mockJSON({
    type: "retirement",
    name: "Fire",
    icon: "retirement",
    savingsTimeline: [
      { date: "2026-10-31T00:00:00Z", value: 20000 },
      { date: "2026-11-01T00:00:00Z", value: 20000 }
    ],
    savingsTotal: 20000,
    investmentTotal: 20000,
    gainTotal: 0,
    targetSavings: 1000000,
    monthlyContribution: 0,
    forecastRate: 0,
    swr: 4,
    yearlyExpense: 40000,
    xirr: 0,
    postings: [posting],
    balances: {
      "Assets:Goal:Retirement": { group: "Assets:Goal:Retirement", marketAmount: 20000 }
    },
    intelligence
  });

  const response = await getRetirementGoal("Fire");
  expect(response.name).toBe("Fire");
  expect(response.swr).toBe(4);
  expect(response.savingsTimeline).toHaveLength(2);
  expect(dayjs.isDayjs(response.savingsTimeline[0].date)).toBe(true);
  expect(response.savingsTimeline[0].date.format("YYYY-MM-DD")).toBe("2026-10-31");
  expect(response.savingsTimeline[1].date.format("YYYY-MM-DD")).toBe("2026-11-01");
  expect(dayjs.isDayjs(response.postings[0].date)).toBe(true);
  expect(response.postings[0].date.format("YYYY-MM-DD")).toBe("2026-10-31");
  expect(response.balances["Assets:Goal:Retirement"].marketAmount).toBe(20000);
  expect(response.intelligence.scheduleStatus).toBe("no-deadline");
});

test("getSavingsGoal keeps the date-only target and normalizes null collections", async () => {
  mockJSON({
    type: "savings",
    name: "Vacation",
    icon: "vacation",
    investmentTotal: 4000,
    savingsTotal: 4000,
    gainTotal: 0,
    savingsTimeline: null,
    target: 10000,
    targetDate: "2027-06-30",
    rate: 0.05,
    paymentPerPeriod: 500,
    xirr: 0,
    postings: null,
    balances: null,
    contributionHistory: null,
    intelligence
  });

  const response = await getSavingsGoal("Vacation");
  expect(response.targetDate).toBe("2027-06-30");
  expect(response.savingsTimeline).toEqual([]);
  expect(response.postings).toEqual([]);
  expect(response.balances).toEqual({});
  expect(response.contributionHistory).toEqual([]);
});
