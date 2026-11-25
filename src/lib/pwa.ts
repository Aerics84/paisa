/**
 * Restricted PWA app-shell support.
 *
 * Only the static application shell is cached. Finance data, authentication
 * and every other `/api` response always come directly from the network.
 */
const UPDATE_PROMPT_KEY = "pwa.updateAvailable";
const UPDATE_RELOAD_KEY = "pwa.reload";

let registrationStarted = false;
let updatePrompted = false;

/** Return whether a pathname belongs to the cacheable application shell. */
export function isCacheable(pathname: string): boolean {
  if (pathname === "/api" || pathname.startsWith("/api/")) {
    return false;
  }
  if (pathname === "/" || pathname === "/index.html" || pathname === "/manifest.webmanifest") {
    return true;
  }
  return pathname.startsWith("/_app/immutable/") || pathname.startsWith("/icons/");
}

/** Ask the waiting service worker to activate immediately. */
export async function applyUpdate(): Promise<void> {
  if (typeof navigator === "undefined" || !navigator.serviceWorker) {
    return;
  }
  const registration = await navigator.serviceWorker.getRegistration();
  registration?.waiting?.postMessage({ type: "SKIP_WAITING" });
}

async function showUpdatePrompt(): Promise<void> {
  if (updatePrompted || typeof document === "undefined") {
    return;
  }
  updatePrompted = true;

  try {
    const [{ toast }, { t }, { get }] = await Promise.all([
      import("bulma-toast"),
      import("./i18n"),
      import("svelte/store")
    ]);
    const translate = get(t);

    const message = document.createElement("div");
    message.className = "is-flex is-align-items-center";

    const text = document.createElement("span");
    text.textContent = translate(UPDATE_PROMPT_KEY);

    const button = document.createElement("button");
    button.type = "button";
    button.className = "button is-small is-link ml-2";
    button.textContent = translate(UPDATE_RELOAD_KEY);
    button.addEventListener("click", () => {
      button.disabled = true;
      void applyUpdate();
    });

    message.append(text, button);
    toast({
      message,
      type: "is-info",
      duration: 60_000,
      dismissible: true,
      closeOnClick: false
    });
  } catch {
    // An update prompt is optional; registration and offline support keep working.
    updatePrompted = false;
  }
}

/**
 * Register the app-shell service worker in production builds only.
 *
 * The development server must never be controlled by an old worker, otherwise
 * Vite's module graph and the cached shell would diverge.
 */
export function registerServiceWorker(): void {
  if (registrationStarted) {
    return;
  }
  if (typeof window === "undefined" || typeof navigator === "undefined") {
    return;
  }
  if (!import.meta.env.PROD || !navigator.serviceWorker) {
    return;
  }

  registrationStarted = true;
  const container = navigator.serviceWorker;
  const hadController = Boolean(container.controller);
  let reloading = false;

  // A first install also fires `controllerchange` when the new worker claims the
  // page. Only reload for an update in an already-controlled tab, and only once.
  container.addEventListener("controllerchange", () => {
    if (!hadController || reloading) {
      return;
    }
    reloading = true;
    window.location.reload();
  });

  void container
    .register("/sw.js")
    .then((registration) => {
      if (registration.waiting && hadController) {
        void showUpdatePrompt();
      }

      registration.addEventListener("updatefound", () => {
        const installing = registration.installing;
        if (!installing) {
          return;
        }
        installing.addEventListener("statechange", () => {
          if (installing.state === "installed" && container.controller) {
            void showUpdatePrompt();
          }
        });
      });
    })
    .catch(() => {
      // Offline support is an enhancement; a failed registration must not
      // affect login, onboarding or normal online usage.
    });
}
