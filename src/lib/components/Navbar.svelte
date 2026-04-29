<script lang="ts">
  import { page } from "$app/stores";
  import Actions from "$lib/components/Actions.svelte";
  import { month, year, dateMax, dateMin, dateRangeOption } from "../../store";
  import {
    cashflowExpenseDepth,
    cashflowExpenseDepthAllowed,
    cashflowIncomeDepth,
    cashflowIncomeDepthAllowed,
    obscure
  } from "../../persisted_store";
  import _ from "lodash";
  import {
    financialYear,
    forEachFinancialYear,
    helpUrl,
    now,
    supportsScheduleAL,
    supportsTaxFeatures,
    supportsGermanyTaxFeatures,
    supportsIndiaTaxFeatures
  } from "$lib/utils";
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import DateRange from "./DateRange.svelte";
  import ThemeSwitcher from "./ThemeSwitcher.svelte";
  import MonthPicker from "./MonthPicker.svelte";
  import Logo from "./Logo.svelte";
  import InputRange from "./InputRange.svelte";

  const readonly = USER_CONFIG.readonly;
  let sidebarIdentity = "";
  let sidebarInitials = "";

  onMount(() => {
    if (get(year) == "") {
      year.set(financialYear(now()));
    }

    const rawToken = localStorage.getItem("token") || "";
    sidebarIdentity = rawToken.split(":")[0] || "";
    sidebarInitials = sidebarIdentity
      .split(/[^A-Za-z0-9]+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join("");
  });

  const recurringIcons = [
    { icon: "fa-circle-check", color: "var(--app-positive)", label: "Cleared" },
    { icon: "fa-circle-check", color: "var(--app-warning)", label: "Cleared late" },
    { icon: "fa-exclamation-triangle", color: "var(--app-danger)", label: "Past due" },
    { icon: "fa-circle-check", color: "var(--app-text-subtle)", label: "Upcoming" }
  ];

  interface Link {
    label: string;
    href: string;
    sidebarHref?: string;
    icon?: string;
    help?: string;
    dateRangeSelector?: boolean;
    monthPicker?: boolean;
    financialYearPicker?: boolean;
    maxDepthSelector?: boolean;
    recurringIcons?: boolean;
    children?: Link[];
    disablePreload?: boolean;
    sidebarHidden?: boolean;
  }

  const links: Link[] = [
    { label: "Overview", href: "/", icon: "fa-house", sidebarHref: "/" },
    {
      label: "Accounts",
      href: "/assets",
      icon: "fa-wallet",
      sidebarHref: "/assets/balance",
      children: [
        { label: "Balance", href: "/balance" },
        { label: "Networth", href: "/networth", dateRangeSelector: true },
        { label: "Investment", href: "/investment" },
        { label: "Gain", href: "/gain" },
        { label: "Allocation", href: "/allocation", help: "allocation-targets" },
        { label: "Analysis", href: "/analysis", help: "analysis" }
      ]
    },
    {
      label: "Transactions",
      href: "/ledger",
      icon: "fa-arrow-right-arrow-left",
      sidebarHref: "/ledger/transaction",
      children: [
        { label: "Import", href: "/import", help: "import" },
        { label: "Editor", href: "/editor", help: "editor", disablePreload: true },
        { label: "Transactions", href: "/transaction", help: "bulk-edit" },
        { label: "Postings", href: "/posting" },
        { label: "Price", href: "/price" }
      ]
    },
    {
      label: "Budgets",
      href: "/expense",
      icon: "fa-lightbulb",
      sidebarHref: "/expense/budget",
      children: [
        { label: "Monthly", href: "/monthly", monthPicker: true, dateRangeSelector: true },
        { label: "Yearly", href: "/yearly", financialYearPicker: true },
        { label: "Budget", href: "/budget", help: "budget", monthPicker: true }
      ]
    },
    {
      label: "Investments",
      href: "/assets",
      icon: "fa-chart-column",
      sidebarHref: "/assets/investment",
      children: [
        { label: "Balance", href: "/balance" },
        { label: "Networth", href: "/networth", dateRangeSelector: true },
        { label: "Investment", href: "/investment" },
        { label: "Gain", href: "/gain" },
        { label: "Allocation", href: "/allocation", help: "allocation-targets" },
        { label: "Analysis", href: "/analysis", help: "analysis" }
      ]
    },
    { label: "Goals", href: "/more/goals", icon: "fa-compass", sidebarHref: "/more/goals" },
    {
      label: "Insights",
      href: "/assets/analysis",
      icon: "fa-clipboard-list",
      sidebarHref: "/assets/analysis"
    },
    {
      label: "Reports",
      href: "/cash_flow",
      icon: "fa-rectangle-list",
      sidebarHref: "/cash_flow/income_statement",
      children: [
        { label: "Income Statement", href: "/income_statement", financialYearPicker: true },
        { label: "Monthly", href: "/monthly", dateRangeSelector: true },
        { label: "Yearly", href: "/yearly", financialYearPicker: true, maxDepthSelector: true },
        {
          label: "Recurring",
          href: "/recurring",
          help: "recurring",
          monthPicker: true,
          recurringIcons: true
        }
      ]
    },
    {
      label: "Expenses",
      href: "/expense",
      icon: "fa-chart-pie",
      sidebarHref: "/expense/monthly",
      sidebarHidden: true,
      children: [
        { label: "Monthly", href: "/monthly", monthPicker: true, dateRangeSelector: true },
        { label: "Yearly", href: "/yearly", financialYearPicker: true },
        { label: "Budget", href: "/budget", help: "budget", monthPicker: true }
      ]
    },
    {
      label: "Liabilities",
      href: "/liabilities",
      icon: "fa-scale-balanced",
      sidebarHref: "/liabilities/balance",
      sidebarHidden: true,
      children: [
        { label: "Balance", href: "/balance" },
        { label: "Credit Cards", href: "/credit_cards", help: "credit-cards" },
        { label: "Repayment", href: "/repayment" },
        { label: "Interest", href: "/interest" }
      ]
    },
    {
      label: "Income",
      href: "/income",
      icon: "fa-sack-dollar",
      sidebarHref: "/income",
      sidebarHidden: true
    },
    {
      label: "More",
      href: "/more",
      icon: "fa-sliders",
      sidebarHref: "/more/config",
      sidebarHidden: true,
      children: [
        { label: "Configuration", href: "/config", help: "config" },
        { label: "Sheets", href: "/sheets", help: "sheets", disablePreload: true },
        { label: "Goals", href: "/goals", help: "goals" },
        { label: "Doctor", href: "/doctor" },
        { label: "Logs", href: "/logs" }
      ]
    }
  ];

  const tax: Link = { label: "Tax", href: "/tax", help: "tax", children: [] };

  if (supportsIndiaTaxFeatures(USER_CONFIG)) {
    tax.children.push(
      { label: "Harvest", href: "/harvest", help: "tax-harvesting" },
      { label: "Capital Gains", href: "/capital_gains", help: "capital-gains" }
    );
  }

  if (supportsGermanyTaxFeatures(USER_CONFIG)) {
    tax.children.push({ label: "Capital Income", href: "/capital_income", help: "tax" });
  }

  if (supportsScheduleAL(USER_CONFIG)) {
    tax.children.push({
      label: "Schedule AL",
      href: "/schedule_al",
      help: "schedule-al",
      financialYearPicker: true
    });
  }

  if (supportsTaxFeatures(USER_CONFIG)) {
    _.last(links).children.push(tax);
  }

  _.last(links).children.push({ label: "About", href: "/about" });

  let selectedLink: Link = null;
  let selectedSubLink: Link = null;
  let selectedSubSubLink: Link = null;

  $: normalizedPath = $page.url.pathname?.replace(/(.+)\/$/, "");
  $: isOverview = normalizedPath === "/";

  $: if (normalizedPath) {
    selectedSubLink = null;
    selectedSubSubLink = null;
    selectedLink = _.find(
      links,
      (l) => normalizedPath == l.href || normalizedPath == l.sidebarHref
    );

    if (!selectedLink) {
      selectedLink = _.find(
        links,
        (l) => !_.isEmpty(l.children) && normalizedPath.startsWith(l.href)
      );

      if (selectedLink) {
        selectedSubLink = _.find(
          selectedLink.children,
          (l) => normalizedPath == selectedLink.href + l.href
        );

        if (!selectedSubLink) {
          selectedSubLink = _.find(selectedLink.children, (l) =>
            normalizedPath.startsWith(selectedLink.href + l.href)
          );

          if (!_.isEmpty(selectedSubLink?.children)) {
            selectedSubSubLink = _.find(selectedSubLink.children, (l) =>
              normalizedPath.startsWith(selectedLink.href + selectedSubLink.href + l.href)
            );
          }
        }
      }
    }
  }

  $: contextTitle =
    selectedSubSubLink?.label || selectedSubLink?.label || selectedLink?.label || "Overview";
  $: currentHour = now().hour();
  $: greeting =
    currentHour < 12 ? "Good morning" : currentHour < 18 ? "Good afternoon" : "Good evening";
  $: contextSubtitle = isOverview
    ? `${greeting}, Arjun`
    : selectedLink && selectedSubLink
      ? `${selectedLink.label} - ${selectedSubLink.label}`
      : "Track your finances with a calmer workspace.";
  $: showHelp = selectedSubLink?.help || selectedLink?.help;
  $: showOverviewMonth = isOverview;
  $: showFilters =
    !!selectedSubLink?.recurringIcons ||
    !!selectedSubLink?.dateRangeSelector ||
    !!selectedLink?.dateRangeSelector ||
    !!selectedSubLink?.monthPicker ||
    !!selectedLink?.monthPicker ||
    !!selectedSubSubLink?.financialYearPicker ||
    !!selectedSubLink?.financialYearPicker ||
    !!selectedLink?.financialYearPicker ||
    (!!selectedSubLink?.maxDepthSelector &&
      ($cashflowExpenseDepthAllowed.max > 1 || $cashflowIncomeDepthAllowed.max > 1));
</script>

<aside class="paisa-shell-sidebar">
  <a href="/" class="paisa-shell-sidebar__brand">
    {#if $obscure}
      <span class="icon is-medium">
        <i class="fas fa-user-secret"></i>
      </span>
    {:else}
      <Logo size={22} />
    {/if}
    <span class="is-size-3 has-text-weight-semibold is-primary-color">Paisa</span>
  </a>

  <nav class="paisa-shell-sidebar__nav">
    {#each links.filter((link) => !link.sidebarHidden) as link}
      <a
        class="paisa-shell-sidebar__link"
        class:is-active={selectedLink?.label === link.label}
        href={link.sidebarHref || link.href}
        data-sveltekit-preload-data={link.disablePreload ? "tap" : "hover"}
      >
        <span class="paisa-shell-sidebar__icon icon">
          <i class={"fas " + link.icon}></i>
        </span>
        <span>{link.label}</span>
      </a>
    {/each}
  </nav>

  <div class="paisa-shell-sidebar__utility">
    <a class="paisa-shell-sidebar__link" href="/more/config">
      <span class="paisa-shell-sidebar__icon icon"><i class="fas fa-gear"></i></span>
      <span>Settings</span>
    </a>
    <a class="paisa-shell-sidebar__link" href="https://paisa.fyi" target="_blank" rel="noreferrer">
      <span class="paisa-shell-sidebar__icon icon"><i class="fas fa-circle-question"></i></span>
      <span>Help</span>
    </a>
  </div>

  {#if sidebarIdentity}
    <div class="paisa-shell-sidebar__footer">
      <div class="paisa-shell-sidebar__profile">
        <div class="paisa-shell-sidebar__avatar">{sidebarInitials || "PW"}</div>
        <div class="paisa-shell-sidebar__profile-name">{sidebarIdentity}</div>
      </div>
    </div>
  {/if}
</aside>

<div class="paisa-topbar" class:paisa-topbar--overview={isOverview}>
  <div class="paisa-topbar__context" class:paisa-topbar__context--overview={isOverview}>
    <div class="paisa-topbar__title-row">
      <h1 class="paisa-topbar__title">{contextTitle}</h1>
      {#if !isOverview && showHelp}
        <a
          href={helpUrl(selectedSubLink?.help || selectedLink?.help)}
          class="icon has-text-grey"
          aria-label="Open help"
        >
          <i class="fas fa-circle-question"></i>
        </a>
      {/if}
    </div>
    <div class="paisa-topbar__subtitle">{contextSubtitle}</div>
  </div>

  <div class="paisa-topbar__context paisa-topbar__context--tools">
    <div class="paisa-topbar__controls">
      {#if showOverviewMonth}
        <MonthPicker compact bind:value={$month} max={$dateMax} min={$dateMin} />
      {/if}
      {#if readonly}
        <span
          class="tag is-danger is-light invertable"
          data-tippy-content="<p>Paisa is in readonly mode</p>">readonly</span
        >
      {/if}
      <ThemeSwitcher />
      <button
        class="button is-small paisa-topbar__icon-button"
        type="button"
        aria-label="Notifications"
      >
        <span class="icon is-small">
          <i class="far fa-bell"></i>
        </span>
      </button>
      {#if !isOverview}
        <Actions />
      {/if}
    </div>

    {#if showFilters}
      <div class="paisa-topbar__filters">
        {#if selectedSubLink?.recurringIcons}
          <div
            class="is-flex is-align-items-center"
            style="gap: 0.85rem; color: var(--app-text-muted);"
          >
            {#each recurringIcons as icon}
              <div class="is-flex is-align-items-center" style="gap: 0.35rem;">
                <span class="icon is-small" style={`color: ${icon.color};`}>
                  <i class={"fas " + icon.icon}></i>
                </span>
                <span class="is-size-7">{icon.label}</span>
              </div>
            {/each}
          </div>
        {/if}

        {#if selectedSubLink?.maxDepthSelector && ($cashflowExpenseDepthAllowed.max > 1 || $cashflowIncomeDepthAllowed.max > 1)}
          <div class="dropdown is-right is-hoverable">
            <div class="dropdown-trigger">
              <button class="button is-small" aria-haspopup="true" aria-label="Open depth filters">
                <span class="icon is-small">
                  <i class="fas fa-sliders"></i>
                </span>
              </button>
            </div>
            <div class="dropdown-menu" role="menu">
              <div class="dropdown-content px-2 py-2">
                <InputRange
                  label="Expenses"
                  bind:value={$cashflowExpenseDepth}
                  allowed={$cashflowExpenseDepthAllowed}
                />
                <InputRange
                  label="Income"
                  bind:value={$cashflowIncomeDepth}
                  allowed={$cashflowIncomeDepthAllowed}
                />
              </div>
            </div>
          </div>
        {/if}

        {#if selectedSubLink?.dateRangeSelector || selectedLink?.dateRangeSelector}
          <DateRange bind:value={$dateRangeOption} dateMin={$dateMin} dateMax={$dateMax} />
        {/if}

        {#if selectedSubLink?.monthPicker || selectedLink?.monthPicker}
          <MonthPicker bind:value={$month} max={$dateMax} min={$dateMin} />
        {/if}

        {#if selectedSubSubLink?.financialYearPicker || selectedSubLink?.financialYearPicker || selectedLink?.financialYearPicker}
          <div class="select is-small">
            <select bind:value={$year}>
              {#each forEachFinancialYear($dateMin, $dateMax).reverse() as fy}
                <option>{financialYear(fy)}</option>
              {/each}
            </select>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>
