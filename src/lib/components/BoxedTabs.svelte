<script lang="ts">
  import _ from "lodash";

  interface Props {
    options: { label: string; value: any }[];
    value?: any;
  }

  let { options, value = $bindable() }: Props = $props();

  $effect(() => {
    if (value && !options.find((option) => option.value === value) && !_.isEmpty(options)) {
      value = _.last(options).value;
    }
  });
</script>

<div class="boxed-tabs">
  {#each options as option}
    <button
      class="boxed-tab {option.value === value ? 'is-active' : ''}"
      type="button"
      aria-pressed={option.value === value}
      onclick={() => (value = option.value)}
    >
      {option.label}
    </button>
  {/each}
</div>
