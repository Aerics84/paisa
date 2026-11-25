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
let promptedWaiting: ServiceWorker | null = null;

/** Return whether a pathname belongs to the cacheable application shell. */
export function isCacheable(pathname: string): boolean {
  if (pathname === "/api" || pathname.startsWith("/api/")) {
    return false;
  }
  if (
    pathname === "/" ||
    pathname === "/index.html" ||
    pathname === "/manifest.webmanifest" ||
    pathname === "/apple-touch-icon.png"
  ) {
    return true;
  }
  return pathname.startsWith("/_app/immutable/") || pathname.startsWith("/icons/");
}

async function skipWaiting(): Promise<boolean> {
  if (typeof navigator === "undefined" || !navigator.serviceWorker) {
    return false;
  }
  const registration = await navigator.serviceWorker.getRegistration();
  const waiting = registration?.waiting;
  if (!waiting) {
    return false;
  }
  waiting.postMessage({ type: "SKIP_WAITING" });
  return true;
}

/** Ask the waiting service worker to activate immediately. */
export async function applyUpdate(): Promise<void> {
  await skipWaiting();
}

/**
 * Show the update toast once per waiting worker. A new waiting worker re-arms
 * the prompt; if the waiter disappears, the next call re-arms it again.
 */
function maybePromptUpdate(registration: ServiceWorkerRegistration): void {
  const waiting = registration.waiting ?? null;
  if (!waiting) {
    promptedWaiting = null;
    updatePrompted = false;
    return;
  }
  if (waiting === promptedWaiting) {
    return;
  }
  promptedWaiting = waiting;
  updatePrompted = false;
  void showUpdatePrompt();
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
      void skipWaiting().then((applied) => {
        // The waiting worker can activate or disappear between the toast and
        // the click. Reload rather than leaving a dead button.
        if (!applied) {
          window.location.reload();
        }
      });
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
      if (hadController) {
        maybePromptUpdate(registration);
      }

      registration.addEventListener("updatefound", () => {
        const installing = registration.installing;
        if (!installing) {
          return;
        }
        installing.addEventListener("statechange", () => {
          if (installing.state === "installed" && container.controller) {
            maybePromptUpdate(registration);
          }
        });
      });
    })
    .catch(() => {
      // Offline support is an enhancement; a failed registration must not
      // affect login, onboarding or normal online usage.
    });
}
