# Paisa Preview Monthly Financial Review Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing `/preview` into a complete, trustworthy monthly financial review that connects actual ledger flows, household surplus/deficit, and selected-month investment performance without conflating them.

**Architecture:** Keep the live net-worth/checking snapshot separate from the selected-month review. Add a period-specific backend response for monthly posting flows and flow-adjusted position performance, then bind all review modules to the same month picker. Reuse the existing yearly Sankey and investment detail views as references/drill-downs; do not add a parallel dashboard or alter yearly/tax pages.

**Tech Stack:** Go, `shopspring/decimal`, GORM, existing ledger/posting and price models, Gin API, Svelte/TypeScript, D3/Sankey, Bun tests.

**Spec:** `docs/superpowers/specs/2026-10-25-paisa-preview-monthly-review-design.md`

## Global Constraints

- The ledger remains the source of truth; do not add mandatory bank/broker sync or modify journal entries.
- `/preview` is the only dashboard surface changed; current live balances are independent of the selected review month.
- Monthly and yearly reports remain separate; do not duplicate annual rates or redesign `More → Tax`.
- Net income follows Paisa's existing definition; expenses exclude `Expenses:Tax` when computing surplus because tax is already deducted from net income.
- `Surplus / Deficit = net_income - net_expense`; it is neither checking-balance change nor a complete wealth reconciliation.
- Sankey flows come from paired postings in the selected calendar month; do not fabricate a residual-cash branch from surplus.
- Selected-month investment performance is position-level, flow-adjusted, includes dividends in total return, and excludes buys/sells as return.
- Every position in one ranking uses the same calendar period, configured base currency, return method and comparable price-coverage criteria.
- Do not show stale/missing prices as zero or use lifetime XIRR as monthly performance.
- If overall price/flow coverage is insufficient, use one compact “not available” state; do not enumerate every excluded position.
- Compute the monthly review as one period-level backend response; do not issue one API request per position.
- No Recent Transactions tile, no standalone tax tile, no RSI/MACD signals, and no automatic investment advice.
- Use existing English UI copy and configured currency/locale formatting.

## Review Focus

- A month with no matching postings is “No recorded activity”; it does not prove the ledger history is complete.
- Multi-posting transactions, multiple sources/destinations, and hierarchical account expansion must not lose or double-count flow value.
- Internal transfers between checking accounts must not change household surplus.
- Buy, partial sell, full sell, dividend, FX, stock split, missing-price, and stale-price cases must not be ranked as zero return.
- The selected month must update every review widget while current balances remain unchanged.
- Historical price carry-forward across weekends/market holidays must be distinguished from genuinely stale quotes.

---

## Files and Responsibilities

- `internal/server/expense.go` – existing transaction-to-account graph; add selected-month graph support only after reconciliation tests establish its contract.
- `internal/server/monthly_review.go` – selected-month summary, graph, and investment-performance response.
- `internal/server/monthly_performance.go` – pure selected-month position-return calculation using the formula approved after Task 2.
- `internal/server/server.go` – register the monthly review API route.
- `internal/server/assets/balance.go` – per-account checking history for the live snapshot charts.
- `internal/server/monthly_review_test.go` – monthly aggregation, empty-state, and API contract tests.
- `internal/server/expense_test.go` – graph pairing, hierarchy and reconciliation tests.
- `internal/server/monthly_performance_test.go` – selected-period return tests after Task 2 locks the formula.
- `internal/service/market_test.go` – as-of price lookup and freshness behavior tests.
- `src/routes/preview/+page.svelte` – selected-month review, live snapshot, Insights, and Goal Progress composition.
- `src/lib/components/PreviewNavbar.svelte` – available month range and one functioning global month control.
- `src/lib/components/dashboard/DashboardSectionHeader.svelte` – remove nonfunctional selectable affordances or bind them to the global month.
- `src/paisa-dashboard-overview.scss` – chart stroke scaling, responsive trends, and layout polish.
- `src/lib/monthly_review.ts` – typed monthly API response and selected-month response guard.
- `src/lib/monthly_review.test.ts` – month selection, trend and availability-state tests.

## Task 1: Inventory and verify the Sankey transaction contract

**Files:**

- Modify: `docs/superpowers/specs/2026-10-25-paisa-preview-monthly-review-design.md` only if the inventory changes the accepted product contract.
- Test: `internal/server/expense_test.go` (create).

**Interfaces:**

- Existing flow builder: `computeHierarchyGraph(postings []posting.Posting) Graph` in `internal/server/expense.go`.
- Existing transaction grouping: `transaction.Build(postings)` in `internal/model/transaction/transaction.go`.
- Existing performance sources: `GetGain()` returns current per-account gain and lifetime XIRR; `GetAccountGain()` returns an account `NetworthTimeline`.
- Existing price lookup: `service.GetUnitPrice(db, commodity, date)` returns the last stored price on or before `date` and does not expose freshness.

- [ ] **Step 1: Trace the current flow path and record its sign/grouping rules.**
  - Trace `/api/expense` through `GetExpense`, `computeHierarchyGraph`, `transaction.Build`, `addLink`, and `renderFlow`.
  - Record how negative/positive postings become source/destination links and how Income/Expenses account hierarchy is expanded in the design spec.
  - Map each `/preview` widget to its current API data and time scope (`/api/dashboard`, `/api/networth`, `/api/gain`, and the date/month stores); mark which values are live and which are selected-period.
  - Record the investment price path from `GetGain`/`GetAccountGain` through `computeNetworthTimeline` and `GetUnitPrice`, including known freshness gaps.
- [ ] **Step 2: Write Sankey characterization and regression tests** in `internal/server/expense_test.go`.
  - `TestComputeHierarchyGraphBalancesSingleTransaction`: one source posting and one destination posting preserve the transaction value.
  - `TestComputeHierarchyGraphHandlesMultipleSourcesAndDestinations`: split values are paired without losing material amounts.
  - `TestComputeHierarchyGraphExpandsIncomeAndExpenseHierarchy`: parent and leaf links represent the same path at different levels and are not counted twice as independent totals.
  - `TestComputeHierarchyGraphKeepsCheckingTransfersInternal`: transfers between checking accounts remain asset flows, not income or expenses.
  - `TestComputeHierarchyGraphSeparatesLoanPrincipalAndInterest`: liability principal and `Expenses:Interest` remain distinct destinations.
  - `TestComputeHierarchyGraphIncludesInvestmentAndLiabilityFlows`: asset transfers, loan postings, taxes and expense destinations remain traceable to their transaction legs.
  - `TestComputeHierarchyGraphPreservesMinorCurrencyAmounts`: a valid €0.05 flow is not silently discarded by the current €0.10 threshold.
- [ ] **Step 3: Run graph tests and confirm the cent-level regression.**

  Run: `go test ./internal/server -run 'TestComputeHierarchyGraph' -count=1`

  Expected: the characterization cases pass; the €0.05 regression fails because `computeHierarchyGraph` currently ignores source remainders at or below €0.10.

- [ ] **Step 4: Compare the graph against ledger transaction semantics.**
  - Verify source/target signs against `transaction.Build` and `Posting.Amount`.
  - Verify the visual graph does not double count hierarchical parent/child links.
  - Verify any rounding tolerance is explicit and consistent with supported currency precision.
- [ ] **Step 5: Implement the minimum cent-safe graph correction and rerun tests.**

  Run: `go test ./internal/server -run 'TestComputeHierarchyGraph' -count=1`

  Expected: PASS; each tested transaction is traceable to its displayed account paths and no material flow is lost.

- [ ] **Step 6: Commit.**

  `git add internal/server/expense.go internal/server/expense_test.go`

  `git commit -m "fix: preserve cent-level Sankey flows"`

## Task 2: Run the selected-month performance feasibility spike (go/no-go)

**Files:**

- Test: `internal/server/monthly_performance_spike_test.go` (temporary candidate comparison harness).
- Test: `internal/server/monthly_performance_test.go` (permanent cases for the selected method).
- Test: `internal/service/market_test.go` (extend).
- Modify: `docs/superpowers/specs/2026-10-25-paisa-preview-monthly-review-design.md` with the validated method, price-age policy and coverage threshold.

**Interfaces:**

- `NetworthTimeline` has daily `InvestmentAmount`, `WithdrawalAmount`, `GainAmount`, and `BalanceAmount` values.
- `GainAmount` is derived from position market value and cumulative contributions/withdrawals; `GetGain()` does not currently expose selected-month per-position performance.
- Dividend postings use `Income:Dividend:*`; the existing gain endpoint does not currently map them into per-position performance.

- [ ] **Step 1: Create synthetic in-memory ledger/price characterization cases** in `internal/server/monthly_performance_spike_test.go` covering:
  - a held position whose price rises with no trades;
  - a mid-month buy that must not count as a return for the time before purchase;
  - a partial sale and a fully closed position;
  - a dividend paid during the selected month (included in total return and separately identified);
  - a stock split or equivalent quantity-changing event;
  - EUR and foreign-currency positions;
  - missing boundary prices, stale carried-forward prices, weekends and market holidays;
  - a running current month where the ledger and investment prices stop at different latest reliable dates.
- [ ] **Step 2: Compare candidate period-return methods against the synthetic cases.**
  - Evaluate time-weighted subperiod returns and a documented Modified Dietz approximation against the available daily price and posting data.
  - Confirm exactly how dividends map from `Income:Dividend:*` accounts to positions. Do not attribute an unidentifiable dividend to a specific holding.
  - Record the formula for both euro gain/loss and percentage return; lifetime XIRR is not a candidate for the selected-month value.
- [ ] **Step 3: Define and test price freshness and coverage rules.**
  - Use the `Price.Date` for the last quote on/before each required valuation boundary.
  - Distinguish weekends/market holidays from stale quotes and set a justified age limit by supported commodity type.
  - Require the same selected-month window, configured base currency, return method and minimum coverage criteria for every ranked position.
  - Define a portfolio-level threshold for showing the ranking. If it is not met, show one concise unavailable state, not a ranked subset presented as complete or a list of every excluded position.
- [ ] **Step 4: Record the go/no-go result in the design spec.**
  - If a validated flow-adjusted total-return method and usable coverage rules exist, lock them in the spec and proceed.
  - If not, stop before UI/API work and return the specific formula/coverage trade-off for product approval; do not substitute lifetime XIRR or raw quote movement.
- [ ] **Step 5: Promote the selected method's cases to permanent tests** in `internal/server/monthly_performance_test.go` and remove the temporary comparison harness.
  - Keep named cases for cash-flow adjustment, dividends, FX, stale/missing prices and unavailable status.
- [ ] **Step 6: Run the feasibility and permanent tests.**

  Run: `go test ./internal/server ./internal/service -run 'Test(MonthlyPerformance|GetUnitPrice|GetMarketPrice)' -count=1`

  Expected: PASS for the chosen method and explicit unavailable cases; the design spec states the chosen formula and data limits.

- [ ] **Step 7: Commit the validated tests and design decision.**

  `git add internal/server/monthly_performance_test.go internal/service/market_test.go docs/superpowers/specs/2026-10-25-paisa-preview-monthly-review-design.md`

  `git commit -m "test: validate selected-month position performance"`

## Task 3: Add the selected-month backend review contract

**Files:**

- Create: `internal/server/monthly_review.go`
- Create: `internal/server/monthly_performance.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/monthly_review_test.go` (create).

**Interfaces:**

- `PositionValuation` contains `Date time.Time`, `MarketValue decimal.Decimal` in default currency, and `PriceDate time.Time` for the actual quote used.
- `PositionCashFlow` contains `Date time.Time`, `Kind string` (`purchase`, `sale`, or `dividend`) and positive `Amount decimal.Decimal` in default currency; `Kind` determines the flow's role in the approved formula.
- `PositionMonthlyInput` contains `Account string`, `PeriodStart time.Time`, `PeriodEnd time.Time`, `DailyValuations []PositionValuation`, and `CashFlows []PositionCashFlow`.
- `MonthlyPositionPerformance` contains `Account string`, `GainLoss decimal.Decimal`, `TotalReturnPercent decimal.Decimal`, `DividendIncome decimal.Decimal`, `NetContributions decimal.Decimal`, `NetWithdrawals decimal.Decimal`, and `PriceAsOf time.Time`.
- Produces: `calculateMonthlyPositionPerformance(input PositionMonthlyInput) (MonthlyPositionPerformance, bool)` using the formula locked in Task 2; `false` means not rankable, not 0%.
- `MonthlyMetricPoint` contains `Month string` (`YYYY-MM`), `Value decimal.Decimal`, `Available bool`, and `IsMonthToDate bool`.
- `MonthlyReview` contains `Month string`, `ActivityState string` (`ready` or `no_recorded_activity`), `PeriodComplete bool` (calendar-only), `NetIncomeAfterTax decimal.Decimal`, `NetTax decimal.Decimal`, `NetExpense decimal.Decimal`, `Surplus decimal.Decimal`, `FlowGraph Graph`, `NetIncomeTrend []MonthlyMetricPoint`, `InvestmentContributions decimal.Decimal`, `DividendIncome decimal.Decimal`, `BestPerformers []MonthlyPositionPerformance`, `WeakestPerformers []MonthlyPositionPerformance`, and `PerformanceState string` (`ready` or `not_available`). JSON names use the snake_case names listed in the design spec.
- `performance_state` never encodes missing history as a zero return.
- Produces: `GetMonthlyReview(db *gorm.DB, month time.Time) MonthlyReview`.
- HTTP route: `GET /api/dashboard/monthly?month=YYYY-MM`; invalid month syntax returns HTTP 400.
- Consumes: Task 1’s reconciled graph builder and Task 2’s validated monthly position-performance helper.

- [ ] **Step 1: Write failing tests** for:
  - `TestGetMonthlyReviewUsesSelectedCalendarMonth` (month-boundary postings included/excluded correctly);
  - `TestGetMonthlyReviewSubtractsTaxExactlyOnce`;
  - `TestGetMonthlyReviewKeepsSurplusSeparateFromInvestmentAndCheckingMovements`;
  - `TestGetMonthlyReviewMarksCurrentMonthInProgress`;
  - `TestGetMonthlyReviewShowsNoRecordedActivityWithoutClaimingCompleteness`;
  - `TestGetMonthlyReviewReturnsFlowGraphFromActualPostings`;
  - `TestMonthlyDashboardRouteRejectsInvalidMonth`.
- [ ] **Step 2: Run the focused tests to verify they fail.**

  Run: `go test ./internal/server -run 'Test(GetMonthlyReview|MonthlyDashboardRoute)' -count=1`

  Expected: FAIL because the endpoint and response are not yet implemented.

- [ ] **Step 3: Implement `PositionMonthlyInput`, `calculateMonthlyPositionPerformance`, `MonthlyReview`, `GetMonthlyReview`, and the route.**
  - Reuse existing income, tax, expense and account-flow logic; do not copy the Sankey or monthly return formula into the UI.
  - Keep `Surplus = net_income - net_expense`; do not subtract investments or liabilities a second time.
  - Return “no recorded activity” for an empty month without claiming the journal is complete.
- [ ] **Step 4: Run focused tests.**

  Run: `go test ./internal/server -run 'Test(GetMonthlyReview|MonthlyDashboardRoute)' -count=1`

  Expected: PASS; the response is period-consistent and matches source postings.

- [ ] **Step 5: Commit.**

  `git add internal/server/monthly_performance.go internal/server/monthly_review.go internal/server/server.go internal/server/monthly_review_test.go`

  `git commit -m "feat: add selected-month financial review data"`

## Task 4: Add current checking-account histories and available-month bounds

**Files:**

- Modify: `internal/server/assets/balance.go`
- Modify: `internal/server/dashboard.go`
- Test: `internal/server/assets/balance_test.go` (create).

**Interfaces:**

- Produces: `CheckingBalancePoint{Month time.Time, Account string, Balance decimal.Decimal}` and `GetCheckingBalanceTimeline(db *gorm.DB) []CheckingBalancePoint` for the latest 12 recorded calendar months.
- Produces: `AvailableMonthRange{From time.Time, To time.Time}` using earliest/latest recorded posting dates; this is a picker bound, not a history-completeness claim.
- Adds `checkingBalanceTimeline` and `availableMonthRange` to the existing `GetDashboard` response; current `checkingBalances` remain unchanged.

- [ ] **Step 1: Write failing tests** for multiple checking accounts, internal transfers, empty months, and account aggregation.
- [ ] **Step 2: Run tests and verify failure.**

  Run: `go test ./internal/server/assets -run 'TestCheckingBalanceTimeline' -count=1`

  Expected: FAIL because account-level history is not currently returned.

- [ ] **Step 3: Implement the account-level history and expose its available date range.**
  - Month totals must be based on recorded checking postings per account.
  - A transfer between checking accounts changes the two account lines but not the total combined cash.
- [ ] **Step 4: Run focused tests and commit.**

  Run: `go test ./internal/server/assets -run 'TestCheckingBalanceTimeline' -count=1`

  Expected: PASS for per-account and aggregate history.

  `git add internal/server/assets/balance.go internal/server/assets/balance_test.go internal/server/dashboard.go`

  `git commit -m "feat: expose checking account history"`

## Task 5: Bind every preview review module to the selected month

**Files:**

- Modify: `src/routes/preview/+page.svelte`
- Modify: `src/lib/components/PreviewNavbar.svelte`
- Modify: `src/lib/components/dashboard/DashboardSectionHeader.svelte`
- Create: `src/lib/monthly_review.ts`
- Test: `src/lib/monthly_review.test.ts` (create).

**Interfaces:**

- Consumes: `/api/dashboard` for live snapshot/history range and `/api/dashboard/monthly?month=YYYY-MM` for the selected review.
- Produces in `src/lib/monthly_review.ts`: `MonthlyReviewResponse` matching the Go JSON contract and `isCurrentMonthlyReview(review: MonthlyReviewResponse | null, selectedMonth: string): boolean` so stale responses are never rendered for a newly selected month.
- `month` store remains the single selected-month source for all period-specific review content; Goal Progress remains a clearly labeled current status, not a historical value.

- [ ] **Step 1: Write failing tests** in `src/lib/monthly_review.test.ts` for matching/mismatching selected months, missing trend points, no-activity state, and unavailable performance; missing values must remain unavailable, not become zero.
- [ ] **Step 2: Run the focused Bun tests.**

  Run: `bun test --preload ./src/happydom.ts src/lib/monthly_review.test.ts`

  Expected: FAIL before the period review helpers exist.

- [ ] **Step 3: Bind the page to the monthly review API.**
  - On month change, refresh every monthly panel from the same response.
  - Keep current net worth and checking values independent of the selected month.
  - Prevent an older asynchronous request from overwriting a more recent month selection.
  - Remove the decorative “This month” selector or make it use the global month picker; do not leave a fake control.
  - Restrict the picker to recorded date bounds while making no claim that those dates represent complete history.
- [ ] **Step 4: Run focused tests and commit.**

  Run: `bun test --preload ./src/happydom.ts src/lib/monthly_review.test.ts`

  Expected: PASS; all month-driven panels follow the selection and live status does not change.

  `git add src/routes/preview/+page.svelte src/lib/components/PreviewNavbar.svelte src/lib/components/dashboard/DashboardSectionHeader.svelte src/lib/monthly_review.ts src/lib/monthly_review.test.ts`

  `git commit -m "feat: bind preview review to selected month"`

## Task 6: Build the complete `/preview` monthly review UI

**Files:**

- Modify: `src/routes/preview/+page.svelte`
- Modify: `src/paisa-dashboard-overview.scss`
- Modify: `src/lib/components/dashboard/DashboardSectionHeader.svelte`
- Test: `src/lib/monthly_review.test.ts` and relevant preview UI tests.

**Interfaces:**

- Consumes: current status, account-level checking series, `MonthlyReview`, and the performance method locked by Task 2.

- [ ] **Step 1: Add component tests** for the live/monthly separation, surplus label/formula hint, Sankey presence, investment ranking/dividend presentation, Insights and adjacent Goal Progress, and concise unavailable state.
- [ ] **Step 2: Compose the screen in the approved hierarchy.**
  - Live net worth and checking charts at top; checking chart supports multiple accounts.
  - Monthly review beneath: net income with a three-month mini-trend, actual posting Sankey, separate surplus/deficit, selected-month investment winners/weakest and contribution/dividend amounts.
  - Current month is explicitly MTD/in progress; historical selections use the full calendar month.
  - Current-month MTD is visually distinguished and receives no misleading positive/negative delta against a full prior month.
  - Insights respond to the selected month; Goal Progress remains current and is labeled as such.
  - Preserve the existing Net Worth Drivers idea inside the Investment section as separate contributions and market movement; do not repeat it as a second summary.
  - Show the price as-of/coverage status compactly within the Investment section, not as a separate dashboard card or per-position exclusion list.
  - Keep other currently useful preview modules only where they do not duplicate the same period facts.
  - No Recent Transactions, no separate tax tile, no repeated “Compared with [month]” text.
- [ ] **Step 3: Correct charts and visual controls.**
  - Use non-scaling/thin strokes for Net Worth, checking and income trends at responsive sizes.
  - Keep negative, neutral and positive changes distinct; avoid “positive trend” copy when a direction is not inherently good/bad.
  - Only actual interactive controls appear interactive.
- [ ] **Step 4: Verify through the browser** with current month, at least two historical months, no activity, insufficient price history, multiple checking accounts and goals/insights present.
- [ ] **Step 5: Commit.**

  `git add src/routes/preview/+page.svelte src/paisa-dashboard-overview.scss src/lib/components/dashboard/DashboardSectionHeader.svelte src/lib/monthly_review.test.ts`

  `git commit -m "feat: build preview monthly financial review"`

## Task 7: Full integration and regression verification

**Files:**

- Modify: `tests/regression.test.ts`
- Create/update: `tests/fixture/eur/monthly_review.json`
- Create/update: `tests/fixture/eur-hledger/monthly_review.json`
- Create/update: `tests/fixture/germany-eu/monthly_review.json`
- Create/update: `tests/fixture/inr/monthly_review.json`
- Create/update: `tests/fixture/inr-beancount/monthly_review.json`
- Create/update: `tests/fixture/inr-hledger/monthly_review.json`
- Modify: `docs/superpowers/specs/2026-10-25-paisa-preview-monthly-review-design.md` only to record validated formula and price coverage decisions.

- [ ] **Step 1: Add regression coverage** for `/api/dashboard/monthly?month=2022-02` in `tests/regression.test.ts`; record a fixture for each supported journal profile and verify existing yearly/cashflow/tax snapshots remain unchanged.
- [ ] **Step 2: Run backend and frontend verification.**

  Run: `go test ./internal/server ./internal/server/assets ./internal/service`

  Run: `bun test --preload ./src/happydom.ts src/lib/monthly_review.test.ts`

  Run: `bun test tests/regression.test.ts`

  Run: `npm run check`

  Run: `npm run build`

  Expected: all targeted tests, checks and build pass. Also run `go test ./...`; investigate any new failures separately from the already observed Windows-only `internal/utils.TestBuildSubPath` path-separator failure.

- [ ] **Step 3: Review the final diff and commit.**

  `git diff --check`

  `git add tests/regression.test.ts tests/fixture/eur/monthly_review.json tests/fixture/eur-hledger/monthly_review.json tests/fixture/germany-eu/monthly_review.json tests/fixture/inr/monthly_review.json tests/fixture/inr-beancount/monthly_review.json tests/fixture/inr-hledger/monthly_review.json docs/superpowers/specs/2026-10-25-paisa-preview-monthly-review-design.md`

  `git commit -m "test: verify preview monthly review integration"`

## Completion Gate

This feature is not complete or ready to ship until the live snapshot, selected-month Sankey, surplus/deficit, checking histories, Insights/Goals, and flow-adjusted investment ranking all work together under one month selection; data-quality states and regression tests pass; and the UI contains no placeholder or decorative controls.
