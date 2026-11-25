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
    financialYear,
    forEachFinancialYear,
    isMobile,
    now,
    supportsScheduleAL,
    supportsTaxFeatures,
    supportsGermanyTaxFeatures,
    supportsIndiaTaxFeatures,
    supportsPortfolioAnalysis
  } from "$lib/utils";
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import CurrencySelect from "./CurrencySelect.svelte";
  import DateRange from "./DateRange.svelte";
  import ThemeSwitcher from "./ThemeSwitcher.svelte";
  import MonthPicker from "./MonthPicker.svelte";
  import Logo from "./Logo.svelte";
  import InputRange from "./InputRange.svelte";
  import { t } from "$lib/i18n";
  import {
    breadcrumbItems,
    normalizeNavigationPath,
    resolveBreadcrumbTrail,
    type NavigationLink
  } from "$lib/navigation";
  import { resolveFinancialYear } from "$lib/financial_year";

  interface Props {
    isBurger?: boolean | null;
  }

  let { isBurger = $bindable(null) }: Props = $props();

  const readonly = USER_CONFIG.readonly;

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
  });

  const RecurringIcons = $derived([
    { icon: "fa-circle-check", color: "success", label: $t("nav.recurring.cleared") },
    { icon: "fa-circle-check", color: "warning-dark", label: $t("nav.recurring.clearedLate") },
    { icon: "fa-exclamation-triangle", color: "danger", label: $t("nav.recurring.pastDue") },
    { icon: "fa-circle-check", color: "grey", label: $t("nav.recurring.upcoming") }
  ]);

  type Link = NavigationLink;

  function buildLinks(): Link[] {
    const links: Link[] = [
      { label: $t("nav.dashboard"), href: "/", hide: true },
      {
        label: $t("nav.cashFlow"),
        href: "/cash_flow",
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
        label: $t("nav.assets"),
        href: "/assets",
        children: [
          { label: $t("nav.balance"), href: "/balance" },
          {
            label: $t("nav.networth"),
            href: "/networth",
            dateRangeSelector: true,
            currencySelector: true
          },
          { label: $t("nav.investment"), href: "/investment" },
          { label: $t("nav.gain"), href: "/gain", currencySelector: true },
          { label: $t("nav.allocation"), href: "/allocation", help: "allocation-targets" },
          { label: $t("nav.analysis"), href: "/analysis", tag: "alpha", help: "analysis" }
        ]
      },
      {
        label: $t("nav.liabilities"),
        href: "/liabilities",
        children: [
          { label: $t("nav.balance"), href: "/balance" },
          { label: $t("nav.creditCards"), href: "/credit_cards", help: "credit-cards" },
          { label: $t("nav.repayment"), href: "/repayment" },
          { label: $t("nav.interest"), href: "/interest" }
        ]
      },
      { label: $t("nav.income"), href: "/income" },
      {
        label: $t("nav.ledger"),
        href: "/ledger",
        children: [
          { label: $t("nav.import"), href: "/import", help: "import" },
          { label: $t("nav.editor"), href: "/editor", help: "editor", disablePreload: true },
          { label: $t("nav.transactions"), href: "/transaction", help: "bulk-edit" },
          { label: $t("nav.postings"), href: "/posting" },
          { label: $t("nav.price"), href: "/price" }
        ]
      },
      {
        label: $t("nav.more"),
        href: "/more",
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

    if (supportsTaxFeatures(USER_CONFIG)) {
      links.splice(links.length - 1, 0, tax);
    }

    if (!supportsPortfolioAnalysis(USER_CONFIG)) {
      const assets = _.find(links, (link) => link.href === "/assets");
      if (assets) {
        assets.children = assets.children.filter((child) => child.href !== "/analysis");
      }
    }

    const about = { label: $t("nav.about"), href: "/about" };
    _.last(links).children.push(about);

    return links;
  }

  let links: Link[] = $derived(buildLinks());

  let openMobileDropdown: string | null = $state(null);

  let currentPath = $state(normalizeNavigationPath(page.url.pathname || ""));
  afterNavigate((navigation) => {
    currentPath = normalizeNavigationPath(navigation.to?.url.pathname || currentPath);
  });

  let trail = $derived(resolveBreadcrumbTrail(currentPath, links));
  let normalizedPath = $derived(trail.normalizedPath);
  let selectedLink: Link = $derived(trail.selectedLink);
  let selectedSubLink: Link = $derived(trail.selectedSubLink);
  let selectedSubSubLink: Link = $derived(trail.selectedSubSubLink);
  let breadcrumb = $derived(breadcrumbItems(trail));

  // On mobile, expand the section that contains the current route so a
  // reload or deep link shows the active navigation immediately.
  $effect(() => {
    openMobileDropdown =
      typeof window !== "undefined" && isMobile() ? (selectedLink?.href ?? null) : null;
  });
</script>

<nav class="navbar px-2 is-transparent" aria-label={$t("nav.mainNavigation")}>
  <div class="navbar-brand">
    <a
      href="/"
      class:is-active={normalizedPath == "/"}
      class="navbar-item is-size-4 has-text-weight-medium"
    >
      {#if $obscure}
        <span class="icon is-small is-size-5">
          <i class="fas fa-user-secret"></i>
        </span><span class="ml-2 is-primary-color">Paisa</span>
      {:else}
        <Logo size={22} /><span class="ml-1 is-primary-color">Paisa</span>
      {/if}
    </a>
    <button
      type="button"
      class="navbar-burger"
      class:is-active={isBurger === true}
      onclick={(event) => {
        event.preventDefault();
        isBurger = !isBurger;
      }}
      aria-label={$t("nav.menu")}
      aria-expanded={isBurger === true}
      data-target="navbarBasicExample"
    >
      <span aria-hidden="true"></span>
      <span aria-hidden="true"></span>
      <span aria-hidden="true"></span>
    </button>

    <div class="navbar-item navbar-actions-row mobile-top-actions">
      {#if readonly}
        <span
          class="tag is-rounded is-danger is-light invertable"
          data-tippy-content={`<p>${$t("nav.readonlyTooltip")}</p>`}>{$t("nav.readonly")}</span
        >
      {/if}
      <a href="/preview" class="button is-small is-light">{$t("nav.preview")}</a>
      <ThemeSwitcher />
      <Actions />
    </div>
  </div>

  <div class="navbar-menu" class:is-active={isBurger === true}>
    <div class="navbar-start">
      {#each links as link}
        {#if _.isEmpty(link.children)}
          {#if !link.hide}
            <a
              class="navbar-item"
              href={link.href}
              data-sveltekit-preload-data={link.disablePreload ? "tap" : "hover"}
              class:is-active={normalizedPath == link.href}>{link.label}</a
            >
          {/if}
        {:else}
          <div
            class="navbar-item has-dropdown is-hoverable"
            class:is-active={openMobileDropdown === link.href}
          >
            <button
              type="button"
              class="navbar-link"
              class:is-active={normalizedPath.startsWith(link.href)}
              aria-haspopup="true"
              aria-expanded={openMobileDropdown === link.href}
              onclick={(event) => {
                event.preventDefault();
                if (isMobile()) {
                  openMobileDropdown = openMobileDropdown === link.href ? null : link.href;
                }
              }}>{link.label}</button
            >
            <div class="navbar-dropdown {!isMobile() && 'is-boxed'}">
              {#each link.children as sublink}
                {@const href = link.href + sublink.href}
                {#if _.isEmpty(sublink.children)}
                  <a
                    class="navbar-item"
                    {href}
                    data-sveltekit-preload-data={sublink.disablePreload ? "tap" : "hover"}
                    class:is-active={normalizedPath.startsWith(href)}>{sublink.label}</a
                  >
                {:else}
                  <div class="nested has-dropdown navbar-item">
                    <span
                      class="navbar-link is-arrowless is-flex is-justify-content-space-between is-active"
                      class:is-active={normalizedPath.startsWith(href)}
                    >
                      <span>{sublink.label}</span>
                      <span class="icon is-small">
                        <i
                          class="fas {isMobile() ? 'fa-angle-down' : 'fa-angle-right'}"
                          aria-hidden="true"
                        ></i>
                      </span>
                    </span>

                    <div class="dropdown-menu">
                      <div class="dropdown-content">
                        {#each sublink.children as subsublink}
                          <a
                            href={href + subsublink.href}
                            class="navbar-item"
                            data-sveltekit-preload-data={subsublink.disablePreload
                              ? "tap"
                              : "hover"}
                            class:is-active={normalizedPath == href + subsublink.href}
                            >{subsublink.label}</a
                          >
                        {/each}
                      </div>
                    </div>
                  </div>
                {/if}
              {/each}
            </div>
          </div>
        {/if}
      {/each}
    </div>
    <div class="navbar-end" style="margin-right: 0.3em">
      <div class="navbar-item">
        <div class="field is-grouped navbar-actions-row menu-actions-row">
          {#if readonly}
            <p class="control">
              <span
                class="mt-1 tag is-rounded is-danger is-light invertable"
                data-tippy-content={`<p>${$t("nav.readonlyTooltip")}</p>`}
                >{$t("nav.readonly")}</span
              >
            </p>
          {/if}

          <p class="control">
            <a href="/preview" class="button is-small is-light">{$t("nav.openPreview")}</a>
          </p>
          <p class="control">
            <ThemeSwitcher />
          </p>
          <p class="control">
            <Actions />
          </p>
        </div>
      </div>
    </div>
  </div>
</nav>

<div class="mt-3 px-3 is-flex is-justify-content-space-between subnav-bar">
  {#if breadcrumb.length > 0}
    <nav
      style="margin-left: 0.73rem;"
      class="breadcrumb has-chevron-separator mb-0 is-small app-breadcrumb"
      aria-label={$t("nav.breadcrumbs")}
    >
      <ul>
        {#each breadcrumb as item}
          <li>
            <span class="is-inactive">{item.label}</span>
            {#if item.tag}
              <span style="font-size: 0.6rem" class="tag is-rounded is-warning">{item.tag}</span>
            {/if}
          </li>
        {/each}
      </ul>
    </nav>
  {/if}

  <div class="mr-3 is-flex subnav-controls" style="gap: 12px">
    {#if selectedSubLink?.recurringIcons}
      <div class="flex gap-5 items-center has-text-grey">
        {#each RecurringIcons as icon}
          <div data-tippy-content="<p>{icon.label}</p>">
            <span class="icon is-small has-text-{icon.color}">
              <i class={"fas " + icon.icon} aria-hidden="true"></i>
            </span>
            <span class="is-hidden-mobile">{icon.label}</span>
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
            aria-label={$t("nav.configureDepth")}
          >
            <span class="icon is-small">
              <i class="fas fa-sliders" aria-hidden="true"></i>
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

    {#if selectedSubLink?.currencySelector || selectedLink?.currencySelector}
      <CurrencySelect />
    {/if}

    {#if selectedSubLink?.dateRangeSelector || selectedLink?.dateRangeSelector}
      <div>
        <DateRange bind:value={$dateRangeOption} dateMin={$dateMin} dateMax={$dateMax} />
      </div>
    {/if}

    {#if selectedSubLink?.monthPicker || selectedLink?.monthPicker}
      <MonthPicker bind:value={$month} max={$dateMax} min={$dateMin} />
    {/if}

    {#if selectedSubSubLink?.financialYearPicker || selectedSubLink?.financialYearPicker || selectedLink?.financialYearPicker}
      <div class="has-text-centered">
        <div class="select is-small">
          <select bind:value={$year} aria-label={$t("nav.financialYear")}>
            {#each years as fy}
              <option>{fy}</option>
            {/each}
          </select>
        </div>
      </div>
    {/if}
  </div>
</div>

<style lang="scss">
  .navbar-actions-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .mobile-top-actions {
    display: none !important;
  }

  @media screen and (max-width: 768px) {
    .navbar-brand {
      flex-wrap: wrap;
      align-items: center;
      width: 100%;
    }

    .mobile-top-actions {
      display: flex !important;
      flex: 1 1 100%;
      width: 100%;
      max-width: 100%;
      margin-left: 0;
      padding: 0.15rem 0 0.1rem 0.15rem;
      gap: 0.35rem;
      flex-wrap: wrap;
    }

    .mobile-top-actions :global(.theme-toggle) {
      --size: 2.5rem;
      min-width: 40px;
      min-height: 40px;
      border-radius: 0.5rem;
    }

    .menu-actions-row {
      display: none !important;
    }

    .subnav-bar {
      flex-wrap: wrap;
      gap: 0.5rem;
      padding-left: 0.75rem !important;
      padding-right: 0.75rem !important;
    }

    .subnav-bar :global(nav.app-breadcrumb) {
      margin-left: 0 !important;
      font-size: 0.95rem !important;
    }

    .subnav-bar :global(nav.app-breadcrumb ul) {
      flex-wrap: wrap;
      row-gap: 0.25rem;
    }

    .subnav-controls {
      flex-wrap: wrap;
      margin-right: 0 !important;
      width: 100%;
      justify-content: flex-start;
      align-items: center;
      gap: 0.5rem !important;
    }
  }
</style>
