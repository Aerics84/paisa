<script lang="ts">
  import { iconify } from "$lib/icon";
  import _ from "lodash";
  import Select from "svelte-select";

  export let allAccounts: string[];
  export let accounts: string[];

  let allAccountItems: { value: string; label: string; created?: boolean }[];
  let accountItems: { value: string; label: string; created?: boolean }[];

  let filterText = "";
  $: allAccountItems = _.map(allAccounts, (account) => ({
    value: account,
    label: account
  }));

  $: accountItems = _.map(accounts, (account) => ({
    value: account,
    label: account
  }));

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
