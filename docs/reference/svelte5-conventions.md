# Svelte 5 conventions

This project runs Svelte 5 in legacy-compatible mode while the frontend migrates
to runes. New and touched components follow the rules below. During the
migration batches (Tasks 4.4/4.5) no component behavior changes; only the
syntax moves.

## Ratchet

`scripts/check-runes.mjs` counts legacy syntax in `src/**/*.svelte` and compares
it with `scripts/runes-budget.json`:

| Metric                  | Meaning                           |
| ----------------------- | --------------------------------- |
| `export let`            | legacy component props            |
| `$:`                    | legacy reactive statements        |
| `on:click`              | legacy event directive            |
| `createEventDispatcher` | legacy component event dispatcher |

```sh
npm run check:runes
```

The command prints the current counts and fails when a count exceeds its budget.
CI runs it after the other gates.

Budgets only go down. After a migration batch, lower them to the measured counts:

```sh
npm run check:runes -- --update-budget
```

The flag refuses to raise a budget; a count above its budget must be migrated,
not budgeted. The counts are measured at the merge base of this task; do not
copy older numbers from planning documents.

## Props: `export let` → `$props()`

```svelte
<!-- before -->
<script lang="ts">
  export let label: string;
  export let value = 0;
</script>

<!-- after -->
<script lang="ts">
  interface Props {
    label: string;
    value?: number;
  }

  let { label, value = 0 }: Props = $props();
</script>
```

Keep the same defaults and optionality. Type the destructured object with a
`Props` interface instead of loose annotations.

## Two-way bindings: `$bindable()`

Props that parents set with `bind:` must be declared with `$bindable()`:

```svelte
<!-- before -->
<script lang="ts">
  export let value = 0;
</script>
<input type="number" bind:value />

<!-- after -->
<script lang="ts">
  interface Props {
    value?: number;
  }

  let { value = $bindable(0) }: Props = $props();
</script>
<input type="number" bind:value />
```

Only mark a prop as bindable when the component writes back to it. Regular
one-way props stay plain destructured values.

## Derived values: `$:` → `$derived`

```svelte
<!-- before -->
<script lang="ts">
  export let amount = 0;
  export let taxRate = 0;
  $: tax = amount * taxRate;
  $: total = amount + tax;
</script>

<!-- after -->
<script lang="ts">
  interface Props {
    amount?: number;
    taxRate?: number;
  }

  let { amount = 0, taxRate = 0 }: Props = $props();
  let tax = $derived(amount * taxRate);
  let total = $derived(amount + tax);
</script>
```

For multi-statement derivations use `$derived.by(() => { ... })`. Derived
values are pure calculations; do not assign to them.

## Side effects: `$:` → `$effect`

```svelte
<!-- before -->
<script lang="ts">
  export let chartData;
  export let renderer;
  $: if (renderer) {
    renderer.draw(chartData);
  }
</script>

<!-- after -->
<script lang="ts">
  let { chartData, renderer }: Props = $props();

  $effect(() => {
    renderer?.draw(chartData);
  });
</script>
```

Use `$effect` only for imperative side effects (charts, DOM integration,
subscriptions). Do not use it to keep a second copy of state in sync; that is a
`$derived` value. Return a cleanup function for subscriptions and observers.

## Events: `on:event` → `onevent`

```svelte
<!-- before -->
<button on:click={() => (open = true)}>Open</button>
<button on:click|preventDefault={submit}>Save</button>

<!-- after -->
<button onclick={() => (open = true)}>Open</button>
<button
  onclick={(event) => {
    event.preventDefault();
    submit();
  }}
>
  Save
</button>
```

Modifiers become explicit code in the handler. Component listeners follow the
same pattern: `on:change` becomes `onchange`, plus the matching callback prop
below. The ratchet currently tracks `on:click`; all `on:` directives are migrated
the same way.

## Component events: `createEventDispatcher` → callback props

```svelte
<!-- before -->
<script lang="ts">
  import { createEventDispatcher } from "svelte";

  const dispatch = createEventDispatcher<{ change: string }>();
</script>
<button on:click={() => dispatch("change", value)}>Pick</button>

<!-- after -->
<script lang="ts">
  interface Props {
    onchange?: (value: string) => void;
  }

  let { onchange }: Props = $props();
</script>
<button onclick={() => onchange?.(value)}>Pick</button>
```

The parent passes a function instead of listening to an event:

```svelte
<Picker onchange={(value) => (selected = value)} />
```

Callback props are optional (`onchange?`) unless every caller must handle the
event. Call them with optional chaining so components without a listener keep
working.

## Stores stay allowed

Stores are not deprecated and are not part of the ratchet. Keep importing them
and read them with the `$store` auto-subscription:

```svelte
<script lang="ts">
  import { selectedCurrency } from "$lib/store";
</script>

<p>{$selectedCurrency}</p>
```

Do not rewrite the store modules during the runes migration. When a value is
derived from a store, wrap the store read in `$derived(...)` like any other
expression.

## Typing

- No new `any` in new or migrated components. Use a `Props` interface and
  concrete types for callback props.
- Callback props are functions: `onchange?: (value: T) => void`, not
  `onchange?: Function`.
- Keep existing exported types and module imports unchanged unless the compiler
  requires a change.

## Conversion checklist

1. Replace `export let` with a typed `$props()` destructure.
2. Add `$bindable()` only for props written through `bind:`.
3. Replace `$:` value calculations with `$derived`/`$derived.by`.
4. Replace `$:` side effects with `$effect` and clean up subscriptions.
5. Replace `on:event` with `onevent`; inline modifiers explicitly.
6. Replace `createEventDispatcher` with callback props.
7. Keep stores and behavior unchanged.
8. Run `npm run check:runes` and, after the batch, `--update-budget`.
