// Subtitles intentionally use <b> for emphasis and <br> for line breaks.
// Escape everything else so a caller can never inject arbitrary markup
// through a label or account name.
export function sanitizeSubtitle(value: string): string {
  return (value || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/&lt;(\/?)b&gt;/g, "<$1b>")
    .replace(/&lt;br\s*\/?&gt;/g, "<br>");
}
