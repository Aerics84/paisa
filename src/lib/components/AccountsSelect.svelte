<script lang="ts">
  /* eslint-disable svelte/prefer-writable-derived -- Select state is mutable by design (create/remove entries). */
  import { iconify } from "$lib/icon";
  import _ from "lodash";
  import Select from "svelte-select";

  interface Props {
    allAccounts: string[];
    accounts?: string[];
  }

  let { allAccounts, accounts = $bindable() }: Props = $props();

  type AccountItem = { value: string; label: string; created?: boolean };

  function toItems(accounts: string[]): AccountItem[] {
    return _.map(accounts, (account) => ({ value: account, label: account }));
  }

  // svelte-ignore state_referenced_locally
  let allAccountItems = $state<AccountItem[]>(toItems(allAccounts));
  let accountItems = $state<AccountItem[]>(toItems(accounts ?? []));

  let filterText = $state("");

  $effect(() => {
    allAccountItems = toItems(allAccounts);
  });

  $effect(() => {
    accountItems = toItems(accounts ?? []);
  });

  function handleFilter(items: any[]) {
    if (accountItems?.find((i) => i.label === filterText)) return;
    if (items.length === 0 && filterText.length > 0) {
      const prev = allAccountItems.filter((i) => !i.created);
      allAccountItems = [...prev, { value: filterText, label: filterText, created: true }];
    }
  }

  function handleChange(value: any, cleared = false) {
    if (cleared) {
      accountItems = _.without(accountItems, value);
    } else {
      accountItems = _.cloneDeep(value);
    }

    accounts = accountItems.map((i) => i.value);
  }
</script>

<Select
  --list-z-index="5"
  multiple
  class="is-small is-expandable custom-icon"
  items={allAccountItems}
  value={accountItems}
  showChevron={true}
  searchable={true}
  clearable={false}
  onchange={(value) => handleChange(value)}
  onclear={(value) => handleChange(value, true)}
  onfilter={(items) => handleFilter(items)}
  bind:filterText
>
  {#snippet selection({ selection })}
    <div>
      <span>{iconify(selection.label as string)}</span>
    </div>
  {/snippet}
  {#snippet item({ item })}
    <div>
      {item.created ? "Add: " : ""}
      {iconify(item.label as string)}
    </div>
  {/snippet}
</Select>
