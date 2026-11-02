<script lang="ts">
  import Modal from "$lib/components/Modal.svelte";
  import { ajax, type AllocationTarget } from "$lib/utils";
  import {
    allocationTargetsToEntries,
    validateAllocationTargetEntries,
    type AllocationTargetEntry
  } from "$lib/allocation_targets";
  import * as toast from "bulma-toast";
  import _ from "lodash";

  let {
    active = $bindable(false),
    targets = [],
    availableAccounts = [],
    onSave = () => {}
  }: {
    active: boolean;
    targets: AllocationTarget[];
    availableAccounts: string[];
    onSave?: () => void;
  } = $props();

  let localTargets = $state<AllocationTargetEntry[]>([]);
  let saving = $state(false);
  let activeAccountPickerIndex = $state<number | null>(null);
  let accountSearch = $state("");

  $effect(() => {
    if (active) {
      if (targets && targets.length > 0) {
        localTargets = allocationTargetsToEntries(targets);
      } else {
        localTargets = [
          { name: "Equity", target: 60, drift: 5, accounts: ["Assets:Equity:*"] },
          { name: "Debt", target: 40, drift: 5, accounts: ["Assets:Debt:*"] }
        ];
      }
      activeAccountPickerIndex = null;
      accountSearch = "";
    }
  });

  const totalPercentage = $derived(_.sumBy(localTargets, (t) => Number(t.target) || 0));
  const is100Percent = $derived(Math.abs(totalPercentage - 100) < 0.01);

  const filteredAccounts = $derived(
    availableAccounts.filter((acc) =>
      acc.toLowerCase().includes(accountSearch.toLowerCase().trim())
    )
  );

  function addTarget() {
    localTargets = [...localTargets, { name: "", target: 0, drift: 5, accounts: [] }];
  }

  function removeTarget(index: number) {
    localTargets = localTargets.filter((_, i) => i !== index);
    if (activeAccountPickerIndex === index) {
      activeAccountPickerIndex = null;
    }
  }

  function removeAccount(targetIndex: number, accountName: string) {
    localTargets[targetIndex].accounts = localTargets[targetIndex].accounts.filter(
      (a) => a !== accountName
    );
  }

  function toggleAccount(targetIndex: number, accountName: string) {
    const current = localTargets[targetIndex].accounts;
    if (current.includes(accountName)) {
      localTargets[targetIndex].accounts = current.filter((a) => a !== accountName);
    } else {
      localTargets[targetIndex].accounts = [...current, accountName];
    }
  }

  function addCustomPattern(targetIndex: number, pattern: string) {
    const trimmed = pattern.trim();
    if (!trimmed) return;
    if (!localTargets[targetIndex].accounts.includes(trimmed)) {
      localTargets[targetIndex].accounts = [...localTargets[targetIndex].accounts, trimmed];
    }
    accountSearch = "";
  }

  async function handleSave() {
    const validationError = validateAllocationTargetEntries(localTargets);
    if (validationError) {
      toast.toast({ message: validationError, type: "is-warning" });
      return;
    }

    saving = true;
    try {
      await ajax("/api/allocation/targets", {
        method: "POST",
        body: JSON.stringify(localTargets)
      });
      toast.toast({ message: "Asset allocation targets saved successfully!", type: "is-success" });
      active = false;
      onSave();
    } catch (e: any) {
      console.error(e);
      toast.toast({ message: e.message || "Failed to save allocation targets", type: "is-danger" });
    } finally {
      saving = false;
    }
  }
</script>

<Modal bind:active width="min(800px, 95vw)">
  <svelte:fragment slot="head" let:close>
    <div class="flex items-center justify-between w-full">
      <div class="flex items-center gap-2">
        <span class="text-xl">🎯</span>
        <h3 class="text-lg font-bold">Manage Asset Allocation Targets</h3>
      </div>
      <button type="button" class="btn btn-sm btn-ghost btn-circle" onclick={close}>✕</button>
    </div>
  </svelte:fragment>

  <svelte:fragment slot="body">
    <div class="space-y-5">
      <!-- Target Balance Bar -->
      <div class="bg-base-200/60 rounded-xl p-4">
        <div class="flex items-center justify-between text-sm font-semibold mb-2">
          <span>Total Allocation:</span>
          <span
            class="font-mono text-base font-bold"
            class:text-success={is100Percent}
            class:text-warning={totalPercentage < 100}
            class:text-error={totalPercentage > 100}
          >
            {totalPercentage}%
            {#if is100Percent}
              <span class="text-xs font-normal text-success ml-1">✓ Balanced</span>
            {:else if totalPercentage < 100}
              <span class="text-xs font-normal text-warning ml-1"
                >({100 - totalPercentage}% unallocated)</span
              >
            {:else}
              <span class="text-xs font-normal text-error ml-1"
                >(Exceeds 100% by {totalPercentage - 100}%)</span
              >
            {/if}
          </span>
        </div>
        <div class="w-full bg-base-300 rounded-full h-2.5 overflow-hidden">
          <div
            class="h-2.5 rounded-full transition-all duration-300"
            class:bg-success={is100Percent}
            class:bg-warning={totalPercentage < 100}
            class:bg-error={totalPercentage > 100}
            style="width: {Math.min(totalPercentage, 100)}%"
          ></div>
        </div>
      </div>

      <!-- Target List -->
      <div class="space-y-4">
        {#each localTargets as target, idx}
          <div class="border border-base-300 rounded-xl p-4 bg-base-100 shadow-sm relative">
            <div class="grid grid-cols-1 md:grid-cols-12 gap-3 items-start">
              <!-- Name -->
              <div class="md:col-span-5">
                <label
                  for={"target-name-" + idx}
                  class="text-xs font-semibold text-base-content/70 block mb-1"
                >
                  Asset Class Name
                </label>
                <input
                  id={"target-name-" + idx}
                  type="text"
                  bind:value={target.name}
                  placeholder="e.g. Equity, Debt, Gold"
                  class="input input-bordered input-sm w-full font-semibold"
                />
              </div>

              <!-- Target % -->
              <div class="md:col-span-3">
                <label
                  for={"target-pct-" + idx}
                  class="text-xs font-semibold text-base-content/70 block mb-1"
                >
                  Target %
                </label>
                <div class="join w-full">
                  <input
                    id={"target-pct-" + idx}
                    type="number"
                    min="0"
                    max="100"
                    step="1"
                    bind:value={target.target}
                    class="input input-bordered input-sm w-full font-mono join-item"
                  />
                  <span class="btn btn-sm btn-disabled join-item">%</span>
                </div>
              </div>

              <!-- Drift Tolerance % -->
              <div class="md:col-span-3">
                <label
                  for={"target-drift-" + idx}
                  class="text-xs font-semibold text-base-content/70 block mb-1"
                >
                  Drift Band (±%)
                </label>
                <div class="join w-full">
                  <input
                    id={"target-drift-" + idx}
                    type="number"
                    min="0"
                    max="25"
                    step="0.5"
                    bind:value={target.drift}
                    class="input input-bordered input-sm w-full font-mono join-item"
                  />
                  <span class="btn btn-sm btn-disabled join-item">±%</span>
                </div>
              </div>

              <!-- Remove button -->
              <div class="md:col-span-1 flex items-center justify-end pt-5">
                <button
                  type="button"
                  class="btn btn-sm btn-ghost btn-circle text-error"
                  onclick={() => removeTarget(idx)}
                  title="Delete asset class"
                >
                  ✕
                </button>
              </div>
            </div>

            <!-- Assigned Accounts -->
            <div class="mt-3 pt-3 border-t border-base-200">
              <div class="flex items-center justify-between mb-2">
                <span class="text-xs font-semibold text-base-content/70">
                  Assigned Accounts / Patterns ({target.accounts.length})
                </span>
                <button
                  type="button"
                  class="btn btn-xs btn-outline btn-primary gap-1"
                  onclick={() => {
                    activeAccountPickerIndex = activeAccountPickerIndex === idx ? null : idx;
                    accountSearch = "";
                  }}
                >
                  {activeAccountPickerIndex === idx ? "Done Selecting" : "+ Select Accounts"}
                </button>
              </div>

              <!-- Account Tags -->
              <div class="flex flex-wrap gap-1.5 min-h-[32px] items-center">
                {#if target.accounts.length === 0}
                  <span class="text-xs text-base-content/40 italic">
                    No accounts mapped yet. Click "+ Select Accounts" to map.
                  </span>
                {:else}
                  {#each target.accounts as acc}
                    <span
                      class="badge badge-neutral badge-sm gap-1 pl-2 pr-1 py-2 font-mono text-[11px]"
                    >
                      {acc}
                      <button
                        type="button"
                        class="hover:text-error ml-1 font-bold"
                        onclick={() => removeAccount(idx, acc)}
                      >
                        ✕
                      </button>
                    </span>
                  {/each}
                {/if}
              </div>

              <!-- Inline Account Picker Popup -->
              {#if activeAccountPickerIndex === idx}
                <div class="mt-3 p-3 bg-base-200/70 rounded-lg border border-base-300">
                  <div class="flex items-center gap-2 mb-2">
                    <input
                      type="text"
                      bind:value={accountSearch}
                      placeholder="Search accounts or type glob (e.g. Assets:Equity:*)..."
                      class="input input-bordered input-xs flex-grow"
                    />
                    {#if accountSearch.trim()}
                      <button
                        type="button"
                        class="btn btn-xs btn-primary whitespace-nowrap"
                        onclick={() => addCustomPattern(idx, accountSearch)}
                      >
                        Add "{accountSearch.trim()}"
                      </button>
                    {/if}
                  </div>

                  <div class="max-h-40 overflow-y-auto space-y-1 pr-1 text-xs">
                    {#if filteredAccounts.length === 0}
                      <div class="text-base-content/60 py-2 text-center">
                        No matching asset accounts found. You can add a custom glob pattern above.
                      </div>
                    {:else}
                      {#each filteredAccounts as acc}
                        <label
                          class="flex items-center gap-2 p-1 rounded hover:bg-base-300/60 cursor-pointer"
                        >
                          <input
                            type="checkbox"
                            checked={target.accounts.includes(acc)}
                            onchange={() => toggleAccount(idx, acc)}
                            class="checkbox checkbox-xs checkbox-primary"
                          />
                          <span class="font-mono truncate">{acc}</span>
                        </label>
                      {/each}
                    {/if}
                  </div>
                </div>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      <!-- Add Target Button -->
      <div>
        <button
          type="button"
          class="btn btn-sm btn-outline btn-block gap-2 border-dashed"
          onclick={addTarget}
        >
          <span>＋</span> Add Target Asset Class
        </button>
      </div>
    </div>
  </svelte:fragment>

  <svelte:fragment slot="foot" let:close>
    <div class="flex items-center justify-between w-full">
      <button type="button" class="btn btn-sm btn-ghost" onclick={close}> Cancel </button>
      <button
        type="button"
        class="btn btn-sm btn-primary"
        disabled={saving || localTargets.length === 0}
        onclick={handleSave}
      >
        {#if saving}
          <span class="loading loading-spinner loading-xs"></span>
        {/if}
        Save Targets
      </button>
    </div>
  </svelte:fragment>
</Modal>
