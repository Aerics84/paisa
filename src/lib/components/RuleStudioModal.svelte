<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "./Modal.svelte";
  import {
    deleteRule as deleteRuleRequest,
    getRules,
    saveRule as saveRuleRequest,
    type ImportRule
  } from "$lib/api/rules";
  import { askConfirm } from "$lib/confirm";
  import { t } from "$lib/i18n";
  import * as toast from "bulma-toast";
  import _ from "lodash";

  let {
    open = $bindable(false),
    accounts = [],
    initialRule = null,
    onruleschanged = () => {}
  }: {
    open: boolean;
    accounts?: string[];
    initialRule?: Partial<ImportRule> | null;
    onruleschanged?: () => void;
  } = $props();

  let rules: ImportRule[] = $state([]);
  let loading: boolean = $state(false);
  let activeTab: "list" | "edit" = $state("list");

  function emptyRule(): ImportRule {
    return {
      name: "",
      priority: 100,
      enabled: true,
      payee_pattern: "",
      payee_match_type: "contains",
      memo_pattern: "",
      memo_match_type: "contains",
      tx_type: "any",
      target_account: accounts[0] || "Expenses:Miscellaneous",
      tags: [],
      flag_for_review: false
    };
  }

  let editingRule: ImportRule = $state(emptyRule());
  let rawTags: string = $state("");

  function matchTypeLabel(value: string): string {
    if (value === "exact") return $t("components.ruleStudio.matchType.exact");
    if (value === "regex") return $t("components.ruleStudio.matchType.regex");
    return $t("components.ruleStudio.matchType.contains");
  }

  function txTypeLabel(value: string): string {
    if (value === "debit") return $t("components.ruleStudio.txType.debit");
    if (value === "credit") return $t("components.ruleStudio.txType.credit");
    return $t("components.ruleStudio.txType.any");
  }

  $effect(() => {
    if (open) {
      untrack(() => {
        loadRules();
        if (initialRule) {
          editingRule = {
            ...emptyRule(),
            ...initialRule,
            name: initialRule.name || "",
            priority: initialRule.priority || 100,
            enabled: initialRule.enabled ?? true,
            target_account: initialRule.target_account || accounts[0] || "Expenses:Miscellaneous",
            tags: initialRule.tags || []
          };
          rawTags = (editingRule.tags || []).join(", ");
          activeTab = "edit";
        }
      });
    }
  });

  async function loadRules() {
    loading = true;
    try {
      const res = await getRules({ background: true });
      rules = res.rules;
    } catch {
      rules = [];
      toast.toast({
        message: $t("components.ruleStudio.loadFailed"),
        type: "is-warning",
        duration: 5000
      });
    } finally {
      loading = false;
    }
  }

  function startCreate() {
    editingRule = emptyRule();
    rawTags = "";
    activeTab = "edit";
  }

  function startEdit(r: ImportRule) {
    editingRule = _.cloneDeep(r);
    rawTags = (editingRule.tags || []).join(", ");
    activeTab = "edit";
  }

  function toOptionalAmount(value: number | null | undefined): number | undefined {
    if (value === null || value === undefined || Number.isNaN(Number(value))) {
      return undefined;
    }
    return Number(value);
  }

  async function saveRule() {
    if (!editingRule.name.trim()) {
      toast.toast({ message: $t("components.ruleStudio.nameRequired"), type: "is-danger" });
      return;
    }
    if (!editingRule.target_account.trim()) {
      toast.toast({ message: $t("components.ruleStudio.targetRequired"), type: "is-danger" });
      return;
    }

    editingRule.tags = rawTags
      .split(",")
      .map((t) => t.trim())
      .filter((t) => t.length > 0);

    const minAmount = toOptionalAmount(editingRule.min_amount);
    const maxAmount = toOptionalAmount(editingRule.max_amount);
    editingRule.min_amount = minAmount;
    editingRule.max_amount = maxAmount;

    if (minAmount !== undefined && maxAmount !== undefined && minAmount > maxAmount) {
      toast.toast({
        message: $t("components.ruleStudio.minMaxInvalid"),
        type: "is-danger"
      });
      return;
    }

    try {
      const res = await saveRuleRequest(editingRule, { background: true });
      if (res.saved) {
        toast.toast({
          message: $t("components.ruleStudio.saved", { name: editingRule.name }),
          type: "is-success"
        });
        await loadRules();
        activeTab = "list";
        onruleschanged();
      } else {
        toast.toast({
          message: res.message || $t("components.ruleStudio.saveFailed"),
          type: "is-danger"
        });
      }
    } catch (e) {
      toast.toast({
        message: $t("components.ruleStudio.saveFailedWithError", {
          error: e instanceof Error ? e.message : String(e)
        }),
        type: "is-danger"
      });
    }
  }

  async function deleteRule(id?: number) {
    if (id == null) return;
    const confirmed = await askConfirm({
      title: $t("components.ruleStudio.deleteTitle"),
      description: $t("components.ruleStudio.deleteDescription"),
      confirmLabel: $t("common.delete"),
      cancelLabel: $t("common.cancel"),
      variant: "danger"
    });
    if (!confirmed) return;

    try {
      await deleteRuleRequest(id, { background: true });
      toast.toast({ message: $t("components.ruleStudio.deleted"), type: "is-success" });
      await loadRules();
      onruleschanged();
    } catch (e) {
      toast.toast({
        message: $t("components.ruleStudio.deleteFailed", {
          error: e instanceof Error ? e.message : String(e)
        }),
        type: "is-danger"
      });
    }
  }

  async function toggleRule(r: ImportRule) {
    r.enabled = !r.enabled;
    try {
      await saveRuleRequest(r, { background: true });
      onruleschanged();
    } catch (e) {
      r.enabled = !r.enabled;
      toast.toast({
        message: $t("components.ruleStudio.updateFailed", {
          error: e instanceof Error ? e.message : String(e)
        }),
        type: "is-danger"
      });
    }
  }
</script>

<Modal bind:active={open} width="min(860px, 100vw)">
  <svelte:fragment slot="head" let:close>
    <div class="is-flex is-align-items-center is-justify-content-space-between" style="width: 100%">
      <div class="is-flex is-align-items-center" style="gap: 0.5rem">
        <span class="icon is-medium has-text-primary">
          <i class="fas fa-wand-magic-sparkles fa-lg"></i>
        </span>
        <h3 class="title is-5 mb-0">{$t("components.ruleStudio.title")}</h3>
      </div>
      <button class="delete" aria-label={$t("dialog.closeModal")} onclick={() => close(null)}
      ></button>
    </div>
  </svelte:fragment>

  <svelte:fragment slot="body">
    <div class="tabs is-boxed mb-4">
      <ul>
        <li class:is-active={activeTab === "list"}>
          <a
            href="#rules-list"
            onclick={(e) => {
              e.preventDefault();
              activeTab = "list";
            }}
          >
            <span class="icon is-small"><i class="fas fa-list"></i></span>
            <span>{$t("components.ruleStudio.rulesTab", { count: rules.length })}</span>
          </a>
        </li>
        <li class:is-active={activeTab === "edit"}>
          <a
            href="#rule-editor"
            onclick={(e) => {
              e.preventDefault();
              if (activeTab !== "edit") startCreate();
            }}
          >
            <span class="icon is-small"><i class="fas fa-plus"></i></span>
            <span
              >{$t(
                editingRule.id ? "components.ruleStudio.editRule" : "components.ruleStudio.newRule"
              )}</span
            >
          </a>
        </li>
      </ul>
    </div>

    {#if activeTab === "list"}
      <div class="is-flex is-justify-content-space-between is-align-items-center mb-3">
        <p class="is-size-7 has-text-grey">
          {$t("components.ruleStudio.description")}
        </p>
        <button class="button is-small is-primary is-light" onclick={startCreate}>
          <span class="icon is-small"><i class="fas fa-plus"></i></span>
          <span>{$t("components.ruleStudio.addRule")}</span>
        </button>
      </div>

      {#if loading}
        <div class="has-text-centered py-5">
          <span class="icon is-large has-text-grey">
            <i class="fas fa-spinner fa-spin fa-2x"></i>
          </span>
        </div>
      {:else if rules.length === 0}
        <div class="notification is-light has-text-centered py-5">
          <p class="has-text-grey mb-2">{$t("components.ruleStudio.empty")}</p>
          <button class="button is-small is-primary" onclick={startCreate}
            >{$t("components.ruleStudio.createFirst")}</button
          >
        </div>
      {:else}
        <div class="rule-list">
          {#each rules as r (r.id)}
            <div
              class="box p-3 mb-2 is-flex is-align-items-center is-justify-content-space-between rule-card"
              class:has-background-light={!r.enabled}
            >
              <div style="flex: 1; min-width: 0">
                <div class="is-flex is-align-items-center mb-1" style="gap: 0.5rem">
                  <span class="tag is-rounded is-small is-dark font-mono">P{r.priority}</span>
                  <strong class="is-size-6">{r.name}</strong>
                  {#if !r.enabled}
                    <span class="tag is-warning is-light is-small"
                      >{$t("components.ruleStudio.disabled")}</span
                    >
                  {/if}
                  {#if r.flag_for_review}
                    <span class="tag is-danger is-light is-small"
                      >{$t("components.ruleStudio.flagReview")}</span
                    >
                  {/if}
                </div>

                <div class="is-size-7 has-text-grey is-flex is-flex-wrap-wrap" style="gap: 0.75rem">
                  {#if r.payee_pattern}
                    <span
                      >{$t("components.ruleStudio.payeeLabel")} <code>{r.payee_pattern}</code>
                      ({matchTypeLabel(r.payee_match_type)})</span
                    >
                  {/if}
                  {#if r.memo_pattern}
                    <span
                      >{$t("components.ruleStudio.memoLabel")} <code>{r.memo_pattern}</code>
                      ({matchTypeLabel(r.memo_match_type)})</span
                    >
                  {/if}
                  {#if r.tx_type && r.tx_type !== "any"}
                    <span class="tag is-light is-small uppercase">{txTypeLabel(r.tx_type)}</span>
                  {/if}
                  {#if r.min_amount != null || r.max_amount != null}
                    <span>
                      {$t("components.ruleStudio.amountLabel")}
                      {r.min_amount != null ? r.min_amount : "*"}
                      –
                      {r.max_amount != null ? r.max_amount : "*"}
                    </span>
                  {/if}
                  <span class="has-text-link">➔ <b>{r.target_account}</b></span>
                  {#if r.tags && r.tags.length > 0}
                    <span>{$t("components.ruleStudio.tagsLabel")} {r.tags.join(" ")}</span>
                  {/if}
                </div>
              </div>

              <div class="is-flex is-align-items-center ml-3" style="gap: 0.5rem">
                <button
                  class="button is-small is-white"
                  title={$t(
                    r.enabled ? "components.ruleStudio.disable" : "components.ruleStudio.enable"
                  )}
                  onclick={() => toggleRule(r)}
                >
                  <span class="icon is-small has-text-grey">
                    <i
                      class="fas {r.enabled
                        ? 'fa-toggle-on has-text-success'
                        : 'fa-toggle-off'} fa-lg"
                    ></i>
                  </span>
                </button>
                <button
                  class="button is-small is-white"
                  title={$t("components.ruleStudio.edit")}
                  onclick={() => startEdit(r)}
                >
                  <span class="icon is-small has-text-info"><i class="fas fa-pen"></i></span>
                </button>
                <button
                  class="button is-small is-white"
                  title={$t("common.delete")}
                  onclick={() => deleteRule(r.id)}
                >
                  <span class="icon is-small has-text-danger"><i class="fas fa-trash"></i></span>
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    {:else if activeTab === "edit"}
      <div class="columns is-multiline">
        <div class="column is-8 py-1">
          <div class="field">
            <label class="label is-small" for="rule-name"
              >{$t("components.ruleStudio.ruleName")}</label
            >
            <div class="control">
              <input
                id="rule-name"
                class="input is-small"
                type="text"
                bind:value={editingRule.name}
                placeholder="e.g. Uber Rides, Netflix Subscription"
              />
            </div>
          </div>
        </div>
        <div class="column is-4 py-1">
          <div class="field">
            <label class="label is-small" for="rule-priority"
              >{$t("components.ruleStudio.priority")}</label
            >
            <div class="control">
              <input
                id="rule-priority"
                class="input is-small"
                type="number"
                bind:value={editingRule.priority}
              />
            </div>
          </div>
        </div>

        <div class="column is-8 py-1">
          <div class="field">
            <label class="label is-small" for="payee-pattern"
              >{$t("components.ruleStudio.payeePattern")}</label
            >
            <div class="control">
              <input
                id="payee-pattern"
                class="input is-small"
                type="text"
                bind:value={editingRule.payee_pattern}
                placeholder="e.g. UBER, AMAZON, WHOLE FOODS"
              />
            </div>
          </div>
        </div>
        <div class="column is-4 py-1">
          <div class="field">
            <label class="label is-small" for="payee-match"
              >{$t("components.ruleStudio.payeeMatchType")}</label
            >
            <div class="control">
              <div class="select is-small is-fullwidth">
                <select id="payee-match" bind:value={editingRule.payee_match_type}>
                  <option value="contains">{$t("components.ruleStudio.matchContains")}</option>
                  <option value="exact">{$t("components.ruleStudio.matchExact")}</option>
                  <option value="regex">{$t("components.ruleStudio.matchRegex")}</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        <div class="column is-8 py-1">
          <div class="field">
            <label class="label is-small" for="memo-pattern"
              >{$t("components.ruleStudio.memoPattern")}</label
            >
            <div class="control">
              <input
                id="memo-pattern"
                class="input is-small"
                type="text"
                bind:value={editingRule.memo_pattern}
                placeholder="e.g. UPI/REFUND/SALARY"
              />
            </div>
          </div>
        </div>
        <div class="column is-4 py-1">
          <div class="field">
            <label class="label is-small" for="memo-match"
              >{$t("components.ruleStudio.memoMatchType")}</label
            >
            <div class="control">
              <div class="select is-small is-fullwidth">
                <select id="memo-match" bind:value={editingRule.memo_match_type}>
                  <option value="contains">{$t("components.ruleStudio.matchContains")}</option>
                  <option value="exact">{$t("components.ruleStudio.matchExact")}</option>
                  <option value="regex">{$t("components.ruleStudio.matchRegex")}</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        <div class="column is-4 py-1">
          <div class="field">
            <label class="label is-small" for="tx-type"
              >{$t("components.ruleStudio.transactionType")}</label
            >
            <div class="control">
              <div class="select is-small is-fullwidth">
                <select id="tx-type" bind:value={editingRule.tx_type}>
                  <option value="any">{$t("components.ruleStudio.txAny")}</option>
                  <option value="debit">{$t("components.ruleStudio.txDebit")}</option>
                  <option value="credit">{$t("components.ruleStudio.txCredit")}</option>
                </select>
              </div>
            </div>
          </div>
        </div>
        <div class="column is-4 py-1">
          <div class="field">
            <label class="label is-small" for="min-amount"
              >{$t("components.ruleStudio.minAmount")}</label
            >
            <div class="control">
              <input
                id="min-amount"
                class="input is-small"
                type="number"
                step="0.01"
                bind:value={editingRule.min_amount}
                placeholder="0.00"
              />
            </div>
          </div>
        </div>
        <div class="column is-4 py-1">
          <div class="field">
            <label class="label is-small" for="max-amount"
              >{$t("components.ruleStudio.maxAmount")}</label
            >
            <div class="control">
              <input
                id="max-amount"
                class="input is-small"
                type="number"
                step="0.01"
                bind:value={editingRule.max_amount}
                placeholder={$t("components.ruleStudio.noLimit")}
              />
            </div>
          </div>
        </div>

        <div class="column is-12 py-1">
          <div class="field">
            <label class="label is-small" for="target-account"
              >{$t("components.ruleStudio.targetAccount")}</label
            >
            <div class="control">
              <input
                id="target-account"
                class="input is-small"
                type="text"
                list="rule-studio-accounts"
                bind:value={editingRule.target_account}
                placeholder="e.g. Expenses:Transport:Taxi, Expenses:Groceries"
              />
              <datalist id="rule-studio-accounts">
                {#each accounts as acc}
                  <option value={acc}>{acc}</option>
                {/each}
              </datalist>
            </div>
          </div>
        </div>

        <div class="column is-8 py-1">
          <div class="field">
            <label class="label is-small" for="rule-tags"
              >{$t("components.ruleStudio.appendTags")}</label
            >
            <div class="control">
              <input
                id="rule-tags"
                class="input is-small"
                type="text"
                bind:value={rawTags}
                placeholder="#commute, #dining"
              />
            </div>
          </div>
        </div>
        <div class="column is-4 py-1 is-flex is-align-items-center">
          <label class="checkbox is-size-7 mt-4">
            <input type="checkbox" bind:checked={editingRule.flag_for_review} />
            {$t("components.ruleStudio.flagForReview")}
          </label>
        </div>
      </div>
    {/if}
  </svelte:fragment>

  <svelte:fragment slot="foot" let:close>
    {#if activeTab === "edit"}
      <button class="button is-primary is-small" onclick={saveRule}
        >{$t("components.ruleStudio.saveRule")}</button
      >
      <button class="button is-small" onclick={() => (activeTab = "list")}
        >{$t("components.ruleStudio.backToList")}</button
      >
    {:else}
      <button class="button is-small" onclick={() => close(null)}>{$t("common.close")}</button>
    {/if}
  </svelte:fragment>
</Modal>

<style lang="scss">
  .rule-card {
    border: 1px solid rgba(127, 127, 127, 0.2);
    border-radius: 6px;
    transition: transform 120ms ease;
  }

  .rule-card:hover {
    border-color: var(--bulma-primary, #485fc7);
  }

  .rule-list {
    max-height: 55vh;
    overflow-y: auto;
  }
</style>
