<script lang="ts">
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";
  import Logo from "$lib/components/Logo.svelte";
  import { setLocale, t } from "$lib/i18n";
  import { authStatus, login } from "$lib/utils";
  import _ from "lodash";
  let username = $state("");
  let password = $state("");

  let invalid = $state(false);
  let invalidErrorMessage = $state("");

  onMount(async () => {
    try {
      const status = await authStatus();
      if (status.locale) {
        setLocale(status.locale);
      }
    } catch {
      // The login form stays usable in the fallback locale when status is
      // unavailable.
    }
  });

  let loginDisabled = $derived(_.isEmpty(username) || _.isEmpty(password));

  function localizedLoginError(error: string): string {
    switch (error) {
      case "Invalid username or password":
        return $t("login.invalidCredentials");
      case "Too many login attempts. Try again later.":
        return $t("login.tooManyAttempts");
      case "invalid request":
        return $t("login.invalidRequest");
      default:
        return error;
    }
  }

  async function tryLogin() {
    if (loginDisabled) return;

    const { success, error } = await login(username, password);
    invalid = !success;
    if (success) {
      goto("/");
    } else if (error) {
      invalidErrorMessage = localizedLoginError(error);
    }
  }
</script>

<section class="section m-0 p-0">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12 p-0">
        <div class="flex justify-center items-center h-screen">
          <div class="box px-5 w-80">
            <div class="flex justify-center items-center mb-2">
              <div class="mt-1 mr-1"><Logo size={32} /></div>
              <div class="is-size-3">
                <a href="https://paisa.fyi" class="is-primary-color">Paisa</a>
              </div>
            </div>
            <form
              onsubmit={(event) => {
                event.preventDefault();
                tryLogin();
              }}
            >
              <div class="field">
                <label for="login-username" class="label">{$t("login.username")}</label>
                <div class="control">
                  <input
                    id="login-username"
                    class="input"
                    type="text"
                    autocomplete="username"
                    bind:value={username}
                  />
                </div>
              </div>

              <div class="field">
                <label for="login-password" class="label">{$t("login.password")}</label>
                <div class="control">
                  <input
                    id="login-password"
                    class="input"
                    type="password"
                    autocomplete="current-password"
                    bind:value={password}
                  />
                </div>
                {#if invalid}
                  <p class="help is-danger">{invalidErrorMessage}</p>
                {/if}
              </div>

              <div class="field is-grouped is-grouped-right">
                <div class="control">
                  <button class="button is-link" disabled={loginDisabled}
                    >{$t("login.submit")}</button
                  >
                </div>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
