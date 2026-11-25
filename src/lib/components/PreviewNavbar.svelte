<script lang="ts">
  import { page } from "$app/state";
  import { afterNavigate } from "$app/navigation";
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
    authStatus,
    financialYear,
    forEachFinancialYear,
    helpUrl,
    now,
    supportsScheduleAL,
    supportsTaxFeatures,
    supportsGermanyTaxFeatures,
    supportsIndiaTaxFeatures,
    supportsPortfolioAnalysis
  } from "$lib/utils";
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import DateRange from "./DateRange.svelte";
  import ThemeSwitcher from "./ThemeSwitcher.svelte";
  import MonthPicker from "./MonthPicker.svelte";
  import Logo from "./Logo.svelte";
  import InputRange from "./InputRange.svelte";
  import { t } from "$lib/i18n";
  import {
    normalizeNavigationPath,
    resolveBreadcrumbTrail,
    type NavigationLink
  } from "$lib/navigation";
  import { resolveFinancialYear } from "$lib/financial_year";

  const prefix = "/preview";
  const readonly = USER_CONFIG.readonly;
  let sidebarIdentity = $state("");
  let sidebarInitials = $state("");

  let years = $derived(forEachFinancialYear($dateMin, $dateMax).reverse().map(financialYear));

  // The shared year starts empty and the range changes once a page has
  // loaded its data, so keep the selection within it (latest by default).
  $effect(() => {
    const next = resolveFinancialYear($year, years);
    if (next !== $year) {
      year.set(next);
    }
  });

  onMount(async () => {
    if (get(year) == "") {
      year.set(financialYear(now()));
    }

    try {
      const status = await authStatus();
      sidebarIdentity = status.username || "";
    } catch {
      sidebarIdentity = "";
    }

    sidebarInitials = sidebarIdentity
      .split(/[^A-Za-z0-9]+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join("");
  });

  const recurringIcons = $derived([
    { icon: "fa-circle-check", color: "var(--app-positive)", label: $t("nav.recurring.cleared") },
    {
      icon: "fa-circle-check",
      color: "var(--app-warning)",
      label: $t("nav.recurring.clearedLate")
    },
    {
      icon: "fa-exclamation-triangle",
      color: "var(--app-danger)",
      label: $t("nav.recurring.pastDue")
    },
    {
      icon: "fa-circle-check",
      color: "var(--app-text-subtle)",
      label: $t("nav.recurring.upcoming")
    }
  ]);

  type Link = NavigationLink;

  function buildLinks(): Link[] {
    const links: Link[] = [
      { label: $t("nav.overview"), href: "/", icon: "fa-house", sidebarHref: "/" },
      {
        label: $t("nav.accounts"),
        href: "/assets",
        icon: "fa-wallet",
        sidebarHref: "/assets/balance",
        children: [
          { label: $t("nav.balance"), href: "/balance" },
          { label: $t("nav.networth"), href: "/networth", dateRangeSelector: true },
          { label: $t("nav.investment"), href: "/investment" },
          { label: $t("nav.gain"), href: "/gain" },
          { label: $t("nav.allocation"), href: "/allocation", help: "allocation-targets" },
          { label: $t("nav.analysis"), href: "/analysis", help: "analysis" }
        ]
      },
      {
        label: $t("nav.transactions"),
        href: "/ledger",
        icon: "fa-arrow-right-arrow-left",
        sidebarHref: "/ledger/transaction",
        children: [
          { label: $t("nav.import"), href: "/import", help: "import" },
          { label: $t("nav.editor"), href: "/editor", help: "editor", disablePreload: true },
          { label: $t("nav.transactions"), href: "/transaction", help: "bulk-edit" },
          { label: $t("nav.postings"), href: "/posting" },
          { label: $t("nav.price"), href: "/price" }
        ]
      },
      {
        label: $t("nav.budgets"),
        href: "/expense",
        icon: "fa-lightbulb",
        sidebarHref: "/expense/budget",
        children: [
          {
            label: $t("nav.monthly"),
            href: "/monthly",
            monthPicker: true,
            dateRangeSelector: true
          },
          { label: $t("nav.yearly"), href: "/yearly", financialYearPicker: true },
          { label: $t("nav.budget"), href: "/budget", help: "budget", monthPicker: true }
        ]
      },
      {
        label: $t("nav.investments"),
        href: "/assets",
        icon: "fa-chart-column",
        sidebarHref: "/assets/investment",
        children: [
          { label: $t("nav.balance"), href: "/balance" },
          { label: $t("nav.networth"), href: "/networth", dateRangeSelector: true },
          { label: $t("nav.investment"), href: "/investment" },
          { label: $t("nav.gain"), href: "/gain" },
          { label: $t("nav.allocation"), href: "/allocation", help: "allocation-targets" },
          { label: $t("nav.analysis"), href: "/analysis", help: "analysis" }
        ]
      },
      {
        label: $t("nav.goals"),
        href: "/more/goals",
        icon: "fa-compass",
        sidebarHref: "/more/goals"
      },
      {
        label: $t("nav.insights"),
        href: "/assets/analysis",
        icon: "fa-clipboard-list",
        sidebarHref: "/assets/analysis"
      },
      {
        label: $t("nav.reports"),
        href: "/cash_flow",
        icon: "fa-rectangle-list",
        sidebarHref: "/cash_flow/income_statement",
        children: [
          {
            label: $t("nav.incomeStatement"),
            href: "/income_statement",
            financialYearPicker: true
          },
          { label: $t("nav.monthly"), href: "/monthly", dateRangeSelector: true },
          {
            label: $t("nav.yearly"),
            href: "/yearly",
            financialYearPicker: true,
            maxDepthSelector: true
          },
          {
            label: $t("nav.recurring"),
            href: "/recurring",
            help: "recurring",
            monthPicker: true,
            recurringIcons: true
          }
        ]
      },
      {
        label: $t("nav.expenses"),
        href: "/expense",
        icon: "fa-chart-pie",
        sidebarHref: "/expense/monthly",
        sidebarHidden: true,
        children: [
          {
            label: $t("nav.monthly"),
            href: "/monthly",
            monthPicker: true,
            dateRangeSelector: true
          },
          { label: $t("nav.yearly"), href: "/yearly", financialYearPicker: true },
          { label: $t("nav.budget"), href: "/budget", help: "budget", monthPicker: true }
        ]
      },
      {
        label: $t("nav.liabilities"),
        href: "/liabilities",
        icon: "fa-scale-balanced",
        sidebarHref: "/liabilities/balance",
        sidebarHidden: true,
        children: [
          { label: $t("nav.balance"), href: "/balance" },
          { label: $t("nav.creditCards"), href: "/credit_cards", help: "credit-cards" },
          { label: $t("nav.repayment"), href: "/repayment" },
          { label: $t("nav.interest"), href: "/interest" }
        ]
      },
      {
        label: $t("nav.income"),
        href: "/income",
        icon: "fa-sack-dollar",
        sidebarHref: "/income",
        sidebarHidden: true
      },
      {
        label: $t("nav.more"),
        href: "/more",
        icon: "fa-sliders",
        sidebarHref: "/more/config",
        sidebarHidden: true,
        children: [
          { label: $t("nav.configuration"), href: "/config", help: "config" },
          { label: $t("nav.sheets"), href: "/sheets", help: "sheets", disablePreload: true },
          { label: $t("nav.goals"), href: "/goals", help: "goals" },
          { label: $t("nav.doctor"), href: "/doctor" },
          { label: $t("nav.backup"), href: "/backup" },
          { label: $t("nav.logs"), href: "/logs" }
        ]
      }
    ];

    const tax: Link = {
      label: $t("nav.tax"),
      href: "/tax",
      icon: "fa-receipt",
      help: "tax",
      children: []
    };

    if (supportsIndiaTaxFeatures(USER_CONFIG)) {
      tax.children.push(
        { label: $t("nav.harvest"), href: "/harvest", help: "tax-harvesting" },
        { label: $t("nav.capitalGains"), href: "/capital_gains", help: "capital-gains" }
      );
    }

    if (supportsGermanyTaxFeatures(USER_CONFIG)) {
      tax.children.push(
        { label: $t("nav.overview"), href: "/overview", help: "tax" },
        { label: $t("nav.capitalIncome"), href: "/capital_income", help: "tax" }
      );
    }

    if (supportsScheduleAL(USER_CONFIG)) {
      tax.children.push({
        label: $t("nav.scheduleAL"),
        href: "/schedule_al",
        help: "schedule-al",
        financialYearPicker: true
      });
    }

    if (supportsTaxFeatures(USER_CONFIG) && tax.children.length > 0) {
      tax.sidebarHref = "/tax" + tax.children[0].href;
      links.splice(links.length - 1, 0, tax);
    }

    if (!supportsPortfolioAnalysis(USER_CONFIG)) {
      for (const link of links) {
        if (link.children) {
          link.children = link.children.filter((child) => child.href !== "/analysis");
        }
      }
      _.remove(links, (link) => link.href === "/assets/analysis");
    }

    _.last(links).children.push({ label: $t("nav.about"), href: "/about" });

    return links;
  }

  let links: Link[] = $derived(buildLinks());

  function withPrefix(path: string) {
    return path === "/" ? prefix : `${prefix}${path}`;
  }

  let currentPath = $state(normalizeNavigationPath(page.url.pathname || ""));
  afterNavigate((navigation) => {
    currentPath = normalizeNavigationPath(navigation.to?.url.pathname || currentPath);
  });

  let trail = $derived(resolveBreadcrumbTrail(currentPath, links, prefix));
  let normalizedPath = $derived(trail.normalizedPath);
  let selectedLink: Link = $derived(trail.selectedLink);
  let selectedSubLink: Link = $derived(trail.selectedSubLink);
  let selectedSubSubLink: Link = $derived(trail.selectedSubSubLink);
  let isOverview = $derived(normalizedPath === "/");

  let contextTitle = $derived(
    selectedSubSubLink?.label || selectedSubLink?.label || selectedLink?.label || $t("nav.overview")
  );
  let currentHour = $derived(now().hour());
  let greeting = $derived(
    currentHour < 12
      ? $t("nav.goodMorning")
      : currentHour < 18
        ? $t("nav.goodAfternoon")
        : $t("nav.goodEvening")
  );
  let contextSubtitle = $derived(
    isOverview
      ? sidebarIdentity
        ? `${greeting}, ${sidebarIdentity}`
        : $t("nav.livePreviewDashboard")
      : selectedLink && selectedSubLink
        ? `${selectedLink.label} - ${selectedSubLink.label}`
        : $t("nav.previewSubtitle")
  );
  let showHelp = $derived(selectedSubLink?.help || selectedLink?.help);
  let showOverviewMonth = $derived(isOverview);
  let showFilters = $derived(
    !!selectedSubLink?.recurringIcons ||
      !!selectedSubLink?.dateRangeSelector ||
      !!selectedLink?.dateRangeSelector ||
      !!selectedSubLink?.monthPicker ||
      !!selectedLink?.monthPicker ||
      !!selectedSubSubLink?.financialYearPicker ||
      !!selectedSubLink?.financialYearPicker ||
      !!selectedLink?.financialYearPicker ||
      (!!selectedSubLink?.maxDepthSelector &&
        ($cashflowExpenseDepthAllowed.max > 1 || $cashflowIncomeDepthAllowed.max > 1))
  );
</script>

<aside class="paisa-shell-sidebar">
  <a href={prefix} class="paisa-shell-sidebar__brand">
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
        href={withPrefix(link.sidebarHref || link.href)}
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
    <a class="paisa-shell-sidebar__link" href={withPrefix("/more/config")}>
      <span class="paisa-shell-sidebar__icon icon"><i class="fas fa-gear"></i></span>
      <span>{$t("nav.settings")}</span>
    </a>
    <a class="paisa-shell-sidebar__link" href="/" data-no-preview-prefix="true">
      <span class="paisa-shell-sidebar__icon icon"><i class="fas fa-arrow-left"></i></span>
      <span>{$t("nav.backToClassic")}</span>
    </a>
    <a class="paisa-shell-sidebar__link" href="https://paisa.fyi" target="_blank" rel="noreferrer">
      <span class="paisa-shell-sidebar__icon icon"><i class="fas fa-circle-question"></i></span>
      <span>{$t("nav.help")}</span>
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
          aria-label={$t("nav.openHelp")}
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
          data-tippy-content={`<p>${$t("nav.readonlyTooltip")}</p>`}>{$t("nav.readonly")}</span
        >
      {/if}
      <ThemeSwitcher />
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
              <button
                class="button is-small"
                aria-haspopup="true"
                aria-label={$t("nav.openDepthFilters")}
              >
                <span class="icon is-small">
                  <i class="fas fa-sliders"></i>
                </span>
              </button>
            </div>
            <div class="dropdown-menu" role="menu">
              <div class="dropdown-content px-2 py-2">
                <InputRange
                  label={$t("nav.expenses")}
                  bind:value={$cashflowExpenseDepth}
                  allowed={$cashflowExpenseDepthAllowed}
                />
                <InputRange
                  label={$t("nav.income")}
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
              {#each years as fy}
                <option>{fy}</option>
              {/each}
            </select>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</div>
