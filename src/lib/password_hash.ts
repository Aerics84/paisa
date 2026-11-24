import { writable } from "svelte/store";

/**
 * Number of password hashing requests currently in flight.
 *
 * The config form hashes a newly typed password asynchronously before writing
 * it to the configuration object. Saving while a hash is pending would
 * serialize the previous (or empty) value, so the config page disables Save
 * while this counter is greater than zero.
 */
export const pendingPasswordHashes = writable(0);

export function beginPasswordHash() {
  pendingPasswordHashes.update((count) => count + 1);
}

export function endPasswordHash() {
  pendingPasswordHashes.update((count) => Math.max(0, count - 1));
}
