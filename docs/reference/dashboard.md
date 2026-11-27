---
description: "The preview dashboard: presets, section visibility, recent transactions, themes and the mobile drawer."
---

# Preview Dashboard

The preview dashboard at `/preview` is the redesigned workspace for reviewing your finances. It
keeps the narrative order of the overview: net worth, the monthly review, insights and goals, then
supporting signals. The Classic dashboard stays available through **Back to Classic** in the
preview navigation.

## Presets

Use the gear button in the dashboard toolbar to open **Customize**. Pick one of the curated presets
to set which sections are visible:

| Preset    | Visible sections                                                            |
| --------- | --------------------------------------------------------------------------- |
| Overview  | Net worth, monthly review, insights and goals, support, recent transactions |
| Cash flow | Net worth, monthly review, support, recent transactions                     |
| Investor  | Net worth, monthly review, insights and goals                               |
| Minimal   | Net worth, monthly review                                                   |

The dashboard footer is always visible.

## Customize

Inside the **Customize** dialog you can:

- choose a preset, or
- toggle individual sections with the checkboxes under the preset row.

Switching a preset discards manual toggles. A manual toggle marks the layout as custom. Use
**Reset to default** to return to the Overview preset, and **Done** to close the dialog.

The layout is stored in this browser only, under the `preview-dashboard-layout-v1` key, and survives
a reload. A corrupted or unavailable storage value silently falls back to the Overview preset.

## Recent transactions

The **Recent transactions** section shows the six latest postings from your ledger as cards. Each
card links to the transaction in the preview. **View all** opens the full preview transaction list.
The section is hidden when the ledger has no transactions yet.

## Themes

The preview follows the theme switcher. Both the light and the dark theme are fully readable,
including secondary text, titles and chart axes. Contrast is covered by unit checks and Axe
accessibility checks.

## Mobile

On viewports up to 1087 pixels wide the sidebar becomes an off-canvas drawer:

- the burger button in the topbar opens and closes it,
- focus moves into the drawer and cycles there while it is open,
- Escape or the backdrop closes it and returns focus to the burger,
- the page behind the drawer is inert and does not scroll.

Reduced-motion preferences disable the slide animation.
