<script lang="ts">
  import Logo from "$lib/components/Logo.svelte";
  import { ajax } from "$lib/utils";
  import { onMount } from "svelte";
  import { t, type MessageKey } from "$lib/i18n";

  const links: Array<{ name: MessageKey; href: string; icon: string }> = [
    {
      name: "more.about.chat",
      href: "https://matrix.to/#/#paisa:matrix.org",
      icon: "fa-solid fa-headset"
    },
    {
      name: "more.about.issue",
      href: "https://github.com/Aerics84/paisa/issues",
      icon: "fas fa-bug"
    },
    {
      name: "more.about.discussions",
      href: "https://github.com/ananthakumaran/paisa/discussions",
      icon: "fa-regular fa-comments"
    },
    {
      name: "more.about.sourceCode",
      href: "https://github.com/Aerics84/paisa",
      icon: "fa-solid fa-code"
    },
    { name: "more.about.documentation", href: "https://paisa.fyi", icon: "fa-solid fa-book" },
    {
      name: "more.about.releases",
      href: "https://github.com/Aerics84/paisa/releases",
      icon: "fa-solid fa-download"
    },
    {
      name: "more.about.demoUpstream",
      href: "https://demo.paisa.fyi",
      icon: "fa-solid fa-video"
    },
    {
      name: "more.about.upstreamProject",
      href: "https://github.com/ananthakumaran/paisa",
      icon: "fa-solid fa-code-branch"
    }
  ];

  let version = $state("");

  function externalLink(url: string) {
    if (window.runtime) {
      window.runtime.BrowserOpenURL(url);
    } else {
      window.open(url, "_blank");
    }
  }

  onMount(async () => {
    ({ version } = await ajax("/api/version"));
  });
</script>

<section class="section">
  <div class="container is-fluid">
    <div class="columns">
      <div class="column is-12">
        <div class="box has-text-centered px-3 mx-auto" style="max-width: 400px;">
          <div><Logo size={128} /></div>
          <div class="is-size-3 is-primary-color">Paisa</div>
          <div>
            {$t("more.about.version")}: <b>{version}</b>
          </div>
          <div class="is-size-7 mt-2">
            {$t("more.about.forkOf")}
            <a
              href="https://github.com/ananthakumaran/paisa"
              onclick={(event) => {
                event.preventDefault();
                externalLink("https://github.com/ananthakumaran/paisa");
              }}>ananthakumaran/paisa</a
            >{$t("more.about.maintainedBy")}
            <a
              href="https://github.com/Aerics84/paisa"
              onclick={(event) => {
                event.preventDefault();
                externalLink("https://github.com/Aerics84/paisa");
              }}>Aerics84</a
            >
          </div>
        </div>

        <div class="box px-3 mx-auto" style="max-width: 400px;">
          <h3 class="is-size-5 mb-1">{$t("more.about.links")}</h3>
          <ul>
            {#each links as link}
              <li>
                <a
                  href={link.href}
                  onclick={(event) => {
                    event.preventDefault();
                    externalLink(link.href);
                  }}
                >
                  <span class="icon is-small">
                    <i class={link.icon}></i>
                  </span>
                  <span>{$t(link.name)}</span>
                </a>
              </li>
            {/each}
          </ul>
        </div>
      </div>
    </div>
  </div>
</section>
