# Paisa-Fork-Review – Stand 25.10.2026

**Frage:** Welche Forks von `ananthakumaran/paisa` hatten in den letzten sechs Monaten neue Commits, was wurde dort geändert, und welche Themen/Commits sind für unseren Fork (`Aerics84/paisa`) relevant und sollten übernommen werden?

## Kurzfazit

- **36 Forks** hatten seit dem 25.04.2026 Pushes – inklusive unserem eigenen (`Aerics84/paisa`). Von den **35 anderen** sind **34 erreichbar** (`sandeepbazar/paisa` ist inzwischen 404).
- **22 der 34 erreichbaren Fremd-Forks** enthalten eigene Commits, die nicht in `upstream/master` sind. **7** haben nur den Upstream-Stand synchronisiert, **3** nur einen Sync-Merge ohne neue eigene Arbeit, **2** nur Bot-Branches (Dependabot/ImgBot).
- Vier Forks tragen den Großteil der Arbeit: **nextxm** (309 eigene Commits), **Sarath234** (142 + Branch-Arbeit), **1Feng** (136) und **Pratap-kute** (69). Nur die ersten drei enthalten für uns direkt verwertbare Änderungen; Pratap ist ein weitgehender Stack-Umbau (Deno/ECharts, ohne Bulma/D3) und für uns nicht als Ganzes übernehmbar.
- Der größte unmittelbare Nutzen liegt **nicht** in den großen Feature-Branches, sondern in etwa **15 kleinen Korrektheits- und Robustheits-Commits**: Zeitzonen/Datum, hledger-`acost`, `FilterByGlob`, Gehaltsklassifikation, interne Transfers, Yahoo-Crumb/GBp, Tooltips, Template-/Import-Fehler, SQLite-WAL, Backup-Pruning, Preis-Cache/XSS.
- Für den laufenden **Monatsrückblick** liefern Sarath (Insights, effektiver Monat, Segment-Sankey) und nextxm (Net-Worth-Konsistenz, FX, Monatsvergleich) wertvolle Ideen und Tests. Eine 1:1-Übernahme verbietet sich aber, weil unsere Spezifikation strenger ist (TWR, Kursfrische, keine erfundenen Nullen, keine falschen Erfolgszustände).
- **Nicht übernehmen:** Indien-/China-spezifische Scraper und Importe, SMS-/Gmail-/Telegram-Agenten, Kite/Coin-Integrationen, Deno/ECharts-Komplettumbau, MCP/LLM-Spielereien, der nextxm-Architekturumbau (Auth/SQLc/Connect/Job-Queue) als Ganzes.

## Methode und Datenstand

- GitHub-API `/repos/ananthakumaran/paisa/forks?per_page=100&sort=newest` (222 Forks), Filter auf `pushed_at >= 2026-04-25` → 36 Forks.
- Für jeden Fork: Vergleich `master...<owner>:<default_branch>` gegen `upstream/master` (`18ecd7d`, 21.10.2026), zusätzlich Prüfung **aller** Branches per `git ls-remote` und Branch-Compares. Reine Upstream-Sync-Merges werden nicht als eigene Arbeit gezählt.
- „Eigene Commits“ = Commits, die nicht in `upstream/master` enthalten sind. Bei großen, rebasten Forks (nextxm) gibt es dieselben Änderungen teils unter mehreren SHAs; die Zahlen sind daher Näherungen.
- Für die inhaltliche Bewertung wurden **alle eigenen Commits** der aktiven Forks angesehen, nicht nur die seit dem Stichtag. Die Spalte „6 Mon.“ zählt ausschließlich die Commits im Fenster; einzelne empfohlene Fixes (z. B. `FilterByGlob`, Verschlüsselung) sind älter, stehen aber in Forks, die im Fenster aktiv waren.
- Unser Stand: `master` = `f738e0c` (25.10.2026) auf Basis von upstream `v0.7.8`, mit laufender Monatsrückblick-Arbeit (`internal/server/monthly_review.go`, `src/lib/monthly_review.ts`). Die Empfehlungen sind mit diesem laufenden Arbeitsstand abzugleichen.
- Zeilennummern in diesem Dokument sind Orientierung; durch die laufende Monatsrückblick-Arbeit können sie sich verschieben. Maßgeblich sind die genannten Funktionen/Dateien.

## Unser Ausgangspunkt (nicht erneut bauen)

Damit die Empfehlungen nicht Doppelarbeit erzeugen, hier der relevante Ist-Stand unseres Forks:

- `/preview` mit Net-Worth-, Checking-, Expense-, Insight- und Goal-Modulen; klassisches Dashboard bleibt separat.
- Gesamt-Sparquote vs. Investitionsquote (V1) auf den Investment-Jahreskarten.
- Laufender Monatsrückblick: `internal/server/monthly_review.go`, `monthly_performance.go`, `src/lib/monthly_review.ts` (Flow-Graph, Nettoeinkommens-Trend, Top/Flop-Performance, Checking-Historie).
- Jahres- und Monats-Sankey, Germany/EU-Import-Templates (CAMT, Trade Republic, Scalable, Germany Broker) und Germany-Tax-Code.
- Svelte 5, Bulma 1, D3, Go/GORM/SQLite; keine Cloud-/Bank-Sync-Abhängigkeit.

Viele Fork-Features (z. B. Saraths Insights, nextxms Monats-Widgets, Prataps Allocation-UI) überschneiden sich damit. Übernommen werden deshalb primär **Korrektheits- und Randfall-Verbesserungen**, nicht komplette UI-Module.

## 1. Landschaft

### 1.1 Große/breite Forks

| Fork                                                      | Letzter Push                             |      Eigene Commits | Schwerpunkt                                                                                                                        | Für uns                                                                                |
| --------------------------------------------------------- | ---------------------------------------- | ------------------: | ---------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| [nextxm/paisa](https://github.com/nextxm/paisa)           | 25.10.2026                               | 309 (169 seit Apr.) | Sankey, FX-Seite, Net-Worth-/Preis-Fixes, Performance, PWA, Auth/Sessions, Doctor V2/V3, Projektion/FIRE, Import, Asset-Allokation | Nur gezielt einzelne Fixes/Ideen; kein Merge des `develop`                             |
| [Sarath234/paisa](https://github.com/Sarath234/paisa)     | 11.08.2026 (Branches), Master 06.06.2026 | 142 + Branch-Arbeit | Insights, Sparquoten-Widget, Net-Worth-Projektion, Rebalancing, Quick Entry, Suche, Segment-Sankey, Statement-/Agent-Funktionen    | Selektiv: Monatslogik, Insights, Projektion, Rebalancing; Agent/Indien nicht           |
| [1Feng/paisa](https://github.com/1Feng/paisa)             | 21.10.2026                               |                 136 | Mehrwährung/FX, Gehaltsklassifikation, Transfers, Import-Framework, China-Scraper, i18n, Mortgage, Receivables                     | Selektiv: generische Korrektheits-Fixes und Import-Framework; China-spezifisches nicht |
| [Pratap-kute/paisa](https://github.com/Pratap-kute/paisa) | 21.09.2026                               |                  69 | Deno, ECharts 6, Design-Tokens, Vorhersagen, Goals, Budgets, Insights, Healthcheck, Backup/Export, Rebalancing, Importer           | Feature-Ideen; Code passt nicht zu unserem Svelte-5/Bulma/D3-Stack                     |

### 1.2 Fokussierte Fix-/Feature-Forks

| Fork                                                                        | Letzter Push | Eigene Commits (6 Mon.) | Was wurde geändert                                                                                                                                              | Empfehlung                                                                  |
| --------------------------------------------------------------------------- | ------------ | ----------------------: | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| [gvijayakumar-16/paisa](https://github.com/gvijayakumar-16/paisa)           | 23.10.2026   |                       9 | Parallele Kursabfragen, UI-Violett, Pakete entfernt, Speed, Einkommen aggregiert, Export (CSV); ältere Branch: Verschlüsselung at rest                          | Export/Preis-Parallelität prüfen; Verschlüsselung nur bei Bedarf            |
| [matthiasdebernardini/paisa](https://github.com/matthiasdebernardini/paisa) | 07.06.2026   |                       7 | Zeitzonen/Datum, exakte Dezimalen, Yahoo-/Recurring-/Upcoming-Fixes, `os.Exit` in Handlern, Ajax-/Import-/Scraper-Härtung, XIRR, Journal-Includes, Swiss-Reskin | **Ja, Fix-Batch priorisiert**; Design nur Inspiration                       |
| [swapnildahiphale/paisa](https://github.com/swapnildahiphale/paisa)         | 31.08.2026   |                      15 | MCP-Server + AI-REST, Import-Renderer/Preview, validiertes Speichern, Dedupe, ICICI-Template, UPI-Narration, CC-Clearing                                        | Generische Import-Validierung/Dedupe später; Indien-Template nicht          |
| [manishtanwar/paisa-app](https://github.com/manishtanwar/paisa-app)         | 22.07.2026   |           9 (+4 Import) | Import-CLI, XLSX-Offset, Vim-Modus, Goal-Allokationen, `FilterByGlob`-Bug, Kite/Coin (Indien), Drilldowns, Navbar-/ResizeObserver-Fixes                         | **Glob- und Stabilitäts-Fixes ja**; Rest überwiegend Indien                 |
| [d093w1z/paisa](https://github.com/d093w1z/paisa)                           | 28.07.2026   |                       5 | Git-basierte Ledger-Versionshistorie (lokales Repo, Credentials, OAuth)                                                                                         | Später: lokale Git-Historie als Backup-Ersatz, ohne Cloud-Zwang             |
| [mbeutelspacher/paisa](https://github.com/mbeutelspacher/paisa)             | 13.07.2026   |                       3 | Leere Checking-Konten ausblenden, „Guthaben“-Bereich, Dev-Umgebung                                                                                              | Einzelne Idee prüfen; widerspricht teils unserer „No-activity“-Entscheidung |
| [coffmant/paisa](https://github.com/coffmant/paisa)                         | 11.09.2026   |                       2 | Journal-gestützte Reconciliation-API + Workflow-Seite                                                                                                           | Später, read-only; passt zu „keine automatischen Buchungen“                 |
| [stan-ely/paisa](https://github.com/stan-ely/paisa)                         | 04.09.2026   |                       1 | Yahoo-Cookie/Crumb-Authentifizierung, Retry, robuste Fehler                                                                                                     | **Ja, mit GBp-Fix kombinieren**                                             |
| [St-Gonne/paisa](https://github.com/St-Gonne/paisa)                         | 22.10.2026   |                       1 | `negate` erhält Dezimalnotation beim Import (BigNumber)                                                                                                         | **Ja, kleiner Import-Fix**                                                  |
| [mrkstwrt/paisa-source](https://github.com/mrkstwrt/paisa-source)           | 05.06.2026   |                       1 | Yahoo GBp/GBX → GBP (Pence-Faktor 0,01)                                                                                                                         | **Ja, wichtig für LSE-ETFs**                                                |
| [sean0407/paisa](https://github.com/sean0407/paisa)                         | 18.10.2026   |                       1 | Zins-Postings nicht als Investment-Basis im Retirement-Ziel                                                                                                     | **Ja, winziger Fix mit Test**                                               |
| [bellussi1/paisa](https://github.com/bellussi1/paisa)                       | 14.10.2026   |                       2 | Tooltip-Zeilen am Viewport kürzen, Summenzeile erhalten                                                                                                         | **Ja, mit Test**                                                            |
| [savinmikhail/paisa-app](https://github.com/savinmikhail/paisa-app)         | 03.09.2026   |                       1 | Gleicher Tooltip-Fix (Alternative)                                                                                                                              | Nur eine Variante übernehmen                                                |
| [vivekmittal/paisa.fyi](https://github.com/vivekmittal/paisa.fyi)           | 17.10.2026   |      2 (+ ältere Perf.) | Net-Worth/XIRR/Timeline in Zusatzwährungen, Währungs-Dropdown, Asset-Pie, parallele Scraper                                                                     | Später im FX-/Mehrwährungs-Spike prüfen                                     |
| [nightconcept/paisa](https://github.com/nightconcept/paisa)                 | 27.08.2026   |                       2 | hledger `acost` zusätzlich zu `aprice`; Dockerfile.hledger aus Source                                                                                           | **`acost` ja**; Docker später                                               |
| [TuDatTr/paisa](https://github.com/TuDatTr/paisa)                           | 04.10.2026   |                       2 | Zwei Doctor-Regeln entfernt; `hide_zero_checking_balances`                                                                                                      | Doctor-Regeln diskutieren; Checking-Option prüfen                           |
| [panuozzo77/paisa](https://github.com/panuozzo77/paisa)                     | 06.06.2026   |                       1 | Monatsverschiebung in Cashflow-Charts (Dayjs-Zeitzone)                                                                                                          | **Ja, Teil des Zeitzonen-Fixes**                                            |
| [ramkrishna757575/paisa](https://github.com/ramkrishna757575/paisa)         | 09.05.2026   |                       2 | MCP-Server und AI-REST-Endpunkte, Datumsfilter                                                                                                                  | Optional/später; nicht Kern                                                 |

### 1.3 Nur Sync, Merge oder Bots

- **Nur Upstream synchronisiert:** `kp-forks/paisa`, `adeyholar/paisa`, `InameChayote/paisa`, `shrvansudhakara/paisa`, `jainChetan81/paisa`, `parthvzala/paisa`, `blombergniklas/paisa`.
- **Nur Sync-Merge, keine neue eigene Arbeit:** `rgksugan/paisa`, `altanozlu/paisa`, `tonypius/paisa-txs`.
- **Bots:** `mikeg98/paisa` (Dependabot), `annihilatorrrr/paisa` (ImgBot), `St-Gonne/paisa` zusätzlich Dependabot.
- **Nicht mehr erreichbar:** `sandeepbazar/paisa` (404).

> Hinweis: `tonypius` (Regex-Account-Regeln), `altanozlu` (ETF-GraphQL) und `rgksugan` (alte Monatsfixes) haben interessante ältere Branch-Arbeit, aber im Sechs-Monats-Fenster keine neuen eigenen Commits. Sie sind unten unter „Beobachten“ erwähnt.

## 2. Sofort übernehmen (P1)

Die Reihenfolge ist bewusst: zuerst Korrektheit unserer Zahlen (besonders für den laufenden Monatsrückblick), dann Robustheit/Performance, dann UI-Politur.

### P1-01 Zeitzonen- und Datums-Korrektheit

**Commits**

- [matthiasdebernardini/paisa@ef719408](https://github.com/matthiasdebernardini/paisa/commit/ef719408) – „transaction dates render/export one day off in other timezones“ (06.06.2026)
- [panuozzo77/paisa@67a2fa8a](https://github.com/panuozzo77/paisa/commit/67a2fa8a) – gleicher Dayjs-Fix für Monatscharts (06.06.2026)

**Was**

- `utils.Now()` läuft in `config.TimeZone()` statt in der Server-Zeitzone; neue `BeginningOfDay()`-Hilfe.
- `RunningBalance` normalisiert den Starttag, damit Postings am Tagesrand nicht in den falschen Bucket fallen.
- `ajax()` parst Datumsstrings als Kalendertag (`value.slice(0, 10)`), nicht als vollen Instant.
- CSV-Export nutzt `format("YYYY-MM-DD")` statt `toISOString()`.

**Warum bei uns**

- Unser Default-Profil ist `Europe/Berlin` (`internal/config/config.go:246`), aber `utils.Now()` nutzt `time.Now()` (`internal/utils/utils.go:129`). Im Docker-/UTC-Betrieb ist „heute“ damit falsch.
- `RunningBalance` startet mit `postings[0].Date` ohne Normalisierung (`internal/accounting/accounting.go:164`).
- `ajax()` konvertiert `2026-06-06T00:00:00+02:00` in die Browser-Zeitzone (`src/lib/utils.ts:915`); der Export nutzt `toISOString()` (`src/lib/export.ts:8`). Das ist genau der Off-by-one-Fehler, der deutsche Nutzer trifft.
- Betrifft Monatsrückblick, Net-Worth-Timeline, Cashflow, CSV-Export und Regressionstests.

**Ziel bei uns:** `internal/utils/utils.go`, `internal/accounting/accounting.go`, `src/lib/utils.ts`, `src/lib/export.ts`. Tests mit Grenzpostings um 00:00 Europe/Berlin und Browser in UTC.

**Aufwand/Risiko:** S, niedrig. `67a2fa8a` ist im Wesentlichen derselbe Fix; nur einmal übernehmen.

### P1-02 hledger-`acost` unterstützen

**Commit:** [nightconcept/paisa@40743ba1](https://github.com/nightconcept/paisa/commit/40743ba1) (26.08.2026)

**Was:** `HLedgerPosting` bekommt zusätzlich `Cost` (`acost`); `buildHLedgerPostings` bevorzugt `acost` vor `aprice` und behandelt `TotalCost`/`UnitCost` wie `TotalPrice`/`UnitPrice`. Tests für moderne hledger-Ausgaben.

**Warum bei uns:** Unser Parser liest nur `aprice` (`internal/ledger/ledger.go:816-833`). Neuere hledger-Versionen liefern Kosten als `acost`; ohne das sind Einstandswerte, Gains, XIRR und ggf. Steuerberechnungen falsch oder null.

**Ziel bei uns:** `internal/ledger/ledger.go` + Tests. Die Workflow-Änderungen aus dem Commit (Release/CI) nicht übernehmen.

**Aufwand/Risiko:** S, niedrig.

### P1-03 `FilterByGlob`-Semantik korrigieren

**Commits**

- [manishtanwar/paisa-app@122f9eb3](https://github.com/manishtanwar/paisa-app/commit/122f9eb3) – positive Muster OR, negative Muster ausschließend (31.03.2026)
- [Sarath234/paisa@b5a3d091](https://github.com/Sarath234/paisa/commit/b5a3d091) – Muster ohne `*` matchen auch Unterkonten (24.05.2026)

**Was:** Statt „sobald irgendein `!` vorkommt, alle Muster AND“ wird sauber getrennt: Positive Muster werden ODER-verknüpft, negative Muster schließen aus; ohne positive Muster gilt „alles außer negativ“. Zusätzlich Prefix-Matching für Muster ohne Wildcard.

**Warum bei uns:** `internal/accounting/accounting.go:40-80` hat die alte Logik. Bei Kombinationen wie `["Assets:*", "!Assets:Bank", "Assets:Bank:Checking"]` ist das Ergebnis heute falsch. Betroffen sind Retirement-/Savings-Ziele, Allocation-Targets und die neue Transfer-Konfiguration.

**Ziel bei uns:** `internal/accounting/accounting.go` + Tests für gemischte Muster, leere Strings und Capital-Gains-Sonderfälle.

**Aufwand/Risiko:** S, **mittel** wegen Semantikänderung. Vor dem Merge Regression-Fixtures laufen lassen und die Prefix-Regel in `docs/reference/config.md` dokumentieren.

### P1-04 Gehaltsklassifikation und konfigurierbare Salary-Accounts

**Commits**

- [1Feng/paisa@df35cad3](https://github.com/1Feng/paisa/commit/df35cad3) – `Income:Payroll` zählt als Gehalt; ganze Account-Segmente statt `HasPrefix` (13.10.2026)
- [1Feng/paisa@b31b02bb](https://github.com/1Feng/paisa/commit/b31b02bb) – `salary_income_accounts`-Konfiguration (20.10.2026)

**Was:** `isSalaryIncome` prüft `Income:Salary` **und** `Income:Payroll` per `utils.IsSameOrParent`; `Income:SalaryOther` ist damit kein Gehalt mehr. Neue optionale Konfiguration: `null` = Legacy-Defaults, `[]` = keine Gehälter, Liste = exakte Ersetzung. Schema, Doku und Tests sind enthalten.

**Warum bei uns:** `internal/server/investment.go:89-100` nutzt `strings.HasPrefix(..., "Income:Salary")`; `internal/server/monthly_review.go:66` ebenso. Deutsche Nutzer buchen oft `Income:Gehalt`, `Income:Payroll` oder `Income:Employment:*`. Unsere neue Gesamt-Sparquote und der Monatsrückblick würden diese Einkommen sonst als „Other Income“ zählen. Lookalike-Prefixe sind ebenfalls ein Bug.

**Ziel bei uns:** `internal/server/investment.go`, `internal/server/monthly_review.go`, `internal/config/config.go` + `schema.json` + Fixtures.

**Aufwand/Risiko:** M (Config-Schema/Fixtures), fachlich niedrig.

### P1-05 Interne Transfers aus Sparquote/Investment-Timeline ausschließen

**Commit:** [1Feng/paisa@80b0ae5f](https://github.com/1Feng/paisa/commit/80b0ae5f) (27.05.2026)

**Was:** Neue Konfiguration `transfer_accounts` (Globs). Eine Transaktion gilt als interner Transfer, wenn **alle** Legs unter `Assets:*` liegen und **mindestens ein** Leg auf ein konfiguriertes Transfer-Konto passt. Solche Transaktionen werden aus Investment-Timeline und Sparquote entfernt, bleiben aber im Cashflow sichtbar. Der Patch lädt dafür die vollständigen Transaktions-Legs nach.

**Warum bei uns:** `GetInvestment` (`internal/server/investment.go:40-42`) schließt Checking und Liabilities-Transaktionen aus, aber nicht Assets→Assets-Umbuchungen (z. B. `Assets:Tagessgeld` → `Assets:Depot`). Diese können die neue Sparquote künstlich erhöhen. `doublestar/v4` ist bei uns bereits in `go.mod`.

**Ziel bei uns:** `internal/accounting/transfer.go` (neu), `internal/config/config.go` + Schema, `internal/server/investment.go`; zusätzlich in der Berechnung der Investment-Beiträge im Monatsrückblick berücksichtigen.

**Aufwand/Risiko:** M, mittel (Config + Tests). Tests: echter Transfer vs. Gehalt/Ausgabe mit einem passenden Leg.

### P1-06 Yahoo: Cookie/Crumb und GBp/Pence

**Commits**

- [stan-ely/paisa@25859f7d](https://github.com/stan-ely/paisa/commit/25859f7d) – Yahoo-Session mit Cookie-Jar, Crumb, Retry bei 401/403/429, robuste Fehler (04.09.2026)
- [mrkstwrt/paisa-source@7f4bd850](https://github.com/mrkstwrt/paisa-source/commit/7f4bd850) – `GBp`/`GBX` → `GBP` mit Faktor 0,01 vor der FX-Umrechnung (05.06.2026)
- [matthiasdebernardini/paisa@10bc64e1](https://github.com/matthiasdebernardini/paisa/commit/10bc64e1) – Yahoo-Nullwerte/Panics, Upcoming-/Recurring-Guards (06.06.2026)

**Warum bei uns:** `internal/scraper/stock/yahoo.go` hat weder Cookie/Crumb noch eine GBp-Normalisierung. LSE-ETFs werden in Pence gehandelt; ohne Normalisierung schlägt die FX-Suche fehl oder der Wert ist um Faktor 100 falsch. Yahoo antwortet zudem zunehmend mit 401/429 auf nackte Requests. Unsere `UnmarshalJSON`-Behandlung für fehlende Closes ist bereits vorhanden (Upstream `402076f`); diesen Teil nicht regressieren.

**Ziel bei uns:** `internal/scraper/stock/yahoo.go` + Tests. Der Crumb-Patch muss an unseren injizierbaren `yahooHTTPClient` angepasst werden, damit Tests offline bleiben.

**Aufwand/Risiko:** M, mittel (Netzwerk, Retry-Logik). GBp-Fix zuerst, Crumb danach.

### P1-07 Tooltip-Kürzung bei großen Ledgern

**Commits**

- [bellussi1/paisa@5ad11187](https://github.com/bellussi1/paisa/commit/5ad11187) + [4ad755ff](https://github.com/bellussi1/paisa/commit/4ad755ff) (14.10.2026)
- Alternative: [savinmikhail/paisa-app@d1f9ee13](https://github.com/savinmikhail/paisa-app/commit/d1f9ee13) (03.09.2026)

**Was:** Tooltip berücksichtigt Viewport-Höhe, kürzt die List auf die letzten Zeilen, zeigt „N earlier rows omitted“ und behält die Summenzeile. Ausgelagert nach `src/lib/tooltip.ts` mit Tests.

**Warum bei uns:** `src/lib/utils.ts:1217` hängt ungekürzt alle Zeilen an; bei vielen Kategorien/Konten verschwindet die Summe aus dem Viewport. Betrifft Expense-, Cashflow- und Sankey-Charts im Monatsrückblick.

**Ziel bei uns:** `src/lib/utils.ts` / neue `src/lib/tooltip.ts`. Eine der beiden Varianten übernehmen, nicht beide.

**Aufwand/Risiko:** S, niedrig.

### P1-08 UI-Stabilität: Navbar und globaler Error-Handler

**Commits**

- [manishtanwar/paisa-app@4295944c](https://github.com/manishtanwar/paisa-app/commit/4295944c) – Null-Check vor `selectedSubLink.children` (10.07.2026)
- [manishtanwar/paisa-app@0220739b](https://github.com/manishtanwar/paisa-app/commit/0220739b) – `formatError(null)` und ResizeObserver-Loop ignorieren (10.07.2026)

**Warum bei uns:** `src/lib/components/Navbar.svelte:207` dereferenziert `selectedSubLink.children` ohne Null-Check; bei Routen ohne passenden Sub-Link (z. B. neue `/preview`-Routen) crasht die Navigation. `src/hooks.client.ts:75/111` crasht bei `null`-Fehlern und zeigt ResizeObserver-Warnungen als Error-Toast.

**Ziel bei uns:** beide Dateien, je 1–5 Zeilen.

**Aufwand/Risiko:** XS, niedrig.

### P1-09 Import/Template-Fehler

**Commits**

- [1Feng/paisa@0ca8d474](https://github.com/1Feng/paisa/commit/0ca8d474) – leere Template-Liste liefert `[]` statt `null`; UI guardet `templates[0]` (28.05.2026)
- [1Feng/paisa@3ea8aab0](https://github.com/1Feng/paisa/commit/3ea8aab0) – nur `.handlebars`-Dateien als Builtins listen (27.05.2026)
- [1Feng/paisa@63b10690](https://github.com/1Feng/paisa/commit/63b10690) – `amount_alignment_column`-Fallback bei partieller `USER_CONFIG` (27.05.2026)
- [St-Gonne/paisa@f046d740](https://github.com/St-Gonne/paisa/commit/f046d740) – `negate` erhält Dezimalnotation via BigNumber (20.09.2026)

**Warum bei uns:** `internal/model/template/template.go:29` gibt bei leerer Liste `nil` zurück; `src/routes/(app)/ledger/import/+page.svelte:49/112` greift ungeprüft auf `templates[0]` zu. `src/lib/journal.ts:25` setzt `amount_alignment_column` ungeprüft. `src/lib/template_helpers.ts` negiert per `parseAmount * -1` und verliert kleine Dezimalstellen – relevant für deutsche Broker-/Bank-CSV-Templates.

**Ziel bei uns:** `internal/model/template/template.go`, Import-Page, `src/lib/journal.ts`, `src/lib/template_helpers.ts` + Tests.

**Aufwand/Risiko:** S, niedrig.

### P1-10 SQLite-WAL und Backup-Pruning

**Commits**

- [Sarath234/paisa@22abf928](https://github.com/Sarath234/paisa/commit/22abf928) – `PRAGMA journal_mode=WAL`, `busy_timeout=5000` (07.05.2026)
- [Sarath234/paisa@43b52794](https://github.com/Sarath234/paisa/commit/43b52794) – nur die letzten 10 Editor-Backups behalten (07.05.2026)

**Warum bei uns:** `internal/utils/utils.go:279` öffnet SQLite ohne PRAGMAs; parallele Sync-/Preview-Requests können „database is locked“ erzeugen. `internal/server/editor.go` erzeugt bei jedem Speichern ein Backup (`:95`, `:122`) und löscht nie; bei häufigem Speichern wächst das Verzeichnis unbegrenzt.

**Ziel bei uns:** `OpenDB`, `SaveFile`/`SaveSheet` + Tests.

**Aufwand/Risiko:** XS, niedrig.

### P1-11 Performance-/Security-Batch von Sarath

**Commits**

- [Sarath234/paisa@6b730f51](https://github.com/Sarath234/paisa/commit/6b730f51) – Batch-Queries, FIFO O(n), DB-Indizes, XSS, Rate-Limit-Key (18.04.2026)
- [Sarath234/paisa@fdb5c1eb](https://github.com/Sarath234/paisa/commit/fdb5c1eb) – Folgefix: `postings`-Variable in `computeCashFlow` (18.04.2026)

**Was (Auszug)**

- `accounting.FIFO` ohne wiederholtes Reslicing (Head-Index statt `available = available[1:]`), weniger Allokationen/Kopien.
- Indizes auf `postings.date`, `postings.account`, `postings.commodity`, `prices.date`, `prices.commodity_name`.
- `computeCashFlow`: eine DB-Abfrage statt sechs, Filterung in-memory.
- Preis-Cache: `sync.Once` + mutable global → atomarer Pointer/Mutex; fehlender Kurs → Warnung statt `log.Fatal`.
- Potenzielle HTML-Injection-Fläche: `{@html}` in `LevelItem.svelte` und `PriceCodeSearchModal.svelte` entfernt.
- Rate-Limit pro Client-IP statt globalem Schlüssel `"user"`.

**Warum bei uns:** Alle genannten Stellen sind bei uns identisch vorhanden: `internal/accounting/accounting.go` (FIFO), `internal/model/posting/posting.go`/`price.go` (keine Indizes), `internal/server/cash_flow.go` (mehrere Queries), `internal/service/market.go` (`sync.Once`, `log.Fatal`), `src/lib/components/LevelItem.svelte:22`, `PriceCodeSearchModal.svelte:99/143`, `internal/server/server.go:478/499`.

**Wichtig:** Der Zwischenstand von `6b730f51` enthält einen Bug (`postings[0]` nach Refactor nicht definiert), der erst mit `fdb5c1eb` behoben wurde. **Nur den Endstand übernehmen** und in getrennte PRs aufteilen (Perf, Security, Cache).

**Aufwand/Risiko:** M, mittel. Indizes brauchen eine Migration/AutoMigrate-Prüfung; XSS-Fix ist trivial.

### P1-12 Retirement-Ziel: Zinsen nicht als Investment-Basis

**Commit:** [sean0407/paisa@6b12ddf0](https://github.com/sean0407/paisa/commit/6b12ddf0) (18.10.2026)

**Was:** Beim Retirement-Detail werden Zins-Postings aus der `CostBalance` ausgeschlossen, bevor der Gain berechnet wird. Test enthalten.

**Warum bei uns:** `internal/server/goal/retirement.go:52-53` nimmt `accounting.CostBalance(savings)` inklusive Zinsen. Dadurch wird der ausgewiesene Gain zu niedrig. Prüfen, ob `goal/savings.go` dasselbe Muster hat.

**Ziel bei uns:** `internal/server/goal/retirement.go` (+ ggf. `savings.go`) + Test.

**Aufwand/Risiko:** XS, niedrig.

### P1-13 Kleine UI-/Chart-Fixes

- [1Feng/paisa@4de04c31](https://github.com/1Feng/paisa/commit/4de04c31) – Gain-Chart: linken Rand an längste Kontobezeichnung anpassen, statt fix 150 px (28.05.2026). Unser `src/lib/gain.ts:38` nutzt fix 150 px; lange deutsche Kontonamen werden abgeschnitten. **XS.**
- [1Feng/paisa@1404e357](https://github.com/1Feng/paisa/commit/1404e357) / [257b131d](https://github.com/1Feng/paisa/commit/257b131d) – `chartWidth`-Null-Guard für Chart-Container (02.06.2026). Relevant, wenn Charts in versteckten Tabs/beim Monatswechsel gerendert werden. **XS/S.**
- [1Feng/paisa@0ca8d474](https://github.com/1Feng/paisa/commit/0ca8d474) – siehe P1-09.

### P1-14 Optional: Import-Dezimalen/Sonderfälle

**Commit:** [matthiasdebernardini/paisa@7c89e0be](https://github.com/matthiasdebernardini/paisa/commit/7c89e0be) (06.06.2026)

**Was:** Exakte Dezimalbehandlung im Ledger-/Template-Code, wissenschaftliche Notation, Spalten jenseits Z, Template-Lint, Spreadsheet-Helfer. Betrifft `internal/ledger/ledger.go`, `src/lib/spreadsheet.ts`, `src/lib/template_editor.ts`, `src/lib/template_helpers.ts`.

**Warum bei uns:** Deutsche Broker-CSVs enthalten oft viele Nachkommastellen und Spalten jenseits Z. Der Patch ist größer als die anderen P1-Fixes und sollte nach P1-09 separat bewertet werden. **M.**

## 3. Roadmap-relevant (P2)

Diese Themen sind wertvoll, aber entweder größer, spezifikationsabhängig oder erst nach dem Monatsrückblick sinnvoll.

### P2-01 Monatsrückblick: Insights, Monatsvergleiche, Segment-Sankey

**Sarath234**

- `427a8b94` – serverseitige Insights (5 Typen) + `4c00145b`, `e96e314b`, `07097d5b`, `41fb6bec`, `4d0a3ce4`, `ddd9f190`, `4bb0381f`
- `d3ac3a9f` – Sparquoten-Widget, mit Folgefixes `3606a5aa`, `bb2c0e11`, `48edea44`, `db9785a1`, `adf3c6c0`, `c37e1d84` (effektiver Monat, Gehaltseingang, Fallback)
- `b56daaef` – Segment-Cashflow, `ec2d9241` Depth-Filter, `90e5c095`/`ac0309cf` Navigation, `4d180431` Multi-Commodity-Merge, `2341640c` Refunds/Returns im Segment-Sankey

**nextxm**

- `910c0309` – Month-over-Month-Spending-Trends
- `7c21e9de` – Multi-Year-Expense/Income-APIs + YoY-Charts
- `2f42d50f` – Monthly-Breakdown-Widget
- `8b43a53e` – Expense-Heatmap / Daily-Seasonality

**Empfehlung:** Nur Algorithmen, Randfall-Tests und Datenformen übernehmen. Unsere Spezifikation ist maßgeblich: TWR statt Lifetime-XIRR, Kursfrische ≤ 7 Tage, keine 0-%-Rendite aus einem einzigen Kurs, keine erfundenen Cash-Reste. Saraths „effektiver Monat“-Heuristik ist eine gute Vorlage für Gehaltseingang, aber nicht ungeprüft übernehmen. Unser `monthly_review.go`/`monthly_performance.go` deckt Flow-Graph, Nettoeinkommens-Trend und Top/Flop bereits ab; Doppelarbeit vermeiden.

### P2-02 Net-Worth-Konsistenz, FX und Preis-Frische

**nextxm**

- `40c23d8c`, `44866748`, `5b2a3f66`, `4ded9496` – Dashboard- vs. Timeline-Net-Worth synchronisieren, heutige Transaktionen/Preise einbeziehen
- `0de4ec5d` – Same-Day-Preis/Rate-Race mit `EndOfDay`-Pivot
- `c1f2cb61` – Preis-Lookup am Tagesende
- `427934c6` – Yahoo-Null-Closes überspringen
- `b2241973` / `89b1b866` – FX-Rates-Seite mit abgeleiteten Kursen
- `38b6ce71` – FX-Zerlegung der Net-Worth-Entwicklung + Currency-Exposure-API/UI
- `3988a1d4` – `currencies`-Config zur Unterscheidung von Währungen und Wertpapieren

**1Feng**

- `59f7ff6b` – Journal-Quotes mit historischer FX-Abdeckung wiederherstellen, Valuation-Notice
- `1da0a8fb` – nicht-positive Cross-Currency-Rates als fehlend markieren
- `4ed3a329` – Multi-Currency mit `base_currency`, FX-Scrapern und Gain-Attribution (großer Branch)

**vivekmittal**

- `6c44eb6f` – Net-Worth/XIRR/Timeline in Zusatzwährungen
- `11488091` – Asset-Pie im Net-Worth

**Empfehlung:** P2-Spike, eine Methode wählen, nicht mischen. Für unseren Monatsrückblick zählen vor allem: konsistente Timeline, EOD-Preise, explizite FX-Abdeckung und „not available“ statt stiller Null. nextxm baut dafür bereits eine Rate-Cache-/`dcPricesTree`-Struktur, die wir nicht haben; der Code ist nicht direkt cherry-pickbar, die Tests und Semantik sind aber wertvoll.

### P2-03 Net-Worth-Projektion / FIRE

**Sarath234**

- `1fd9353a` – `monthlySavings` (12-Monats-Durchschnitt) in `/api/networth`
- `31d86c3a` – D3-Projektionsrenderer; `4d67ae3e` – Projektionsseite; `48f697c3` – letzte 12 Monate tatsächlich + „Today“-Marker; `8ab1a0e5` – Start an echtem Heute statt Monatsanfang

**nextxm**

- `434223c4` – Net-Worth-Projektion + FIRE-Rechner
- `7acb00a3`, `47316754`, `80c5e7e1`, `e55bb858` – Simulations-API, Life Goals, What-If-Szenarien, tax-aware Drawdown

**Empfehlung:** Sarath ist die kleinere, verständlichere Variante und passt zu unseren bestehenden Goals. nextxm ist ein eigenes Produktmodul (Szenarien, Drawdown, Inflation) – erst nach eigener Spezifikation. Nicht in den Monatsrückblick einbauen.

### P2-04 Rebalancing / Allocation-Targets

**Sarath234**

- `d0aae4ad` – Rebalancing-Rechner auf der Allocation-Seite
- `2658958e`, `36e0a143`, `b5a3d091`, `6e5b9940`, `9b68574d` – Toleranz, Null-Guards, Parent-Prefix-Matching, Leaf-Market-Amount, Managed-Assets-Basis

**Weitere**

- nextxm `feature/assetalloc` (Branch, Okt. 2026) und `04f40522` – Asset-Allocation/Rebalance
- Pratap `04f40522` – gleiches Thema, aber ECharts/Deno-Stack
- vivek `d7719bc7` – Allocation-Target im UI deaktivierbar

**Empfehlung:** Saraths Variante als Basis, weil sie auf unserer Allocation-Seite und D3 aufsetzt. Die Leaf-Market-Amount-Fixes sind auch ohne Rebalancing nützlich.

### P2-05 Import-Framework und Validierung

**1Feng**

- `e7636237` – pluggable Importer (Registry, Stub, `/api/import`-Preview, UI)
- `cbc6fc43` – Review-Fixes
- `1e7eaed9`, `763fde4a`, `eb6e0041`, `eb8b0f19` – China-spezifische Importer (nicht übernehmen)

**swapnildahiphale**

- `519dfd49` – CSV-Handlebars-Renderer für Import
- `08673844` – generische Template-Liste/Preview
- `98cdce40` – validiertes Speichern mit Dedupe
- `40ea4d2d` – Fehler, wenn Journal-Sync keine Postings lädt
- `577ed9fc`, `8547aaa1`, `96cf1e98` – Narrations-Parsing, CC-Dedupe, Clearing-Konto (Indien-spezifisch, aber Muster relevant)

**manishtanwar**

- `bb750e6e` – `paisa import`-CLI; `c0ffa351` – XLSX-Spaltenoffset

**Empfehlung:** Unser Deutschland/EU-Import ist ein Differenzierungsmerkmal. Zuerst Preview + Validierung + Dedupe (ohne neue Importer), dann CLI. Keine Indien-Templates.

### P2-06 Doctor / Datenqualität

- 1Feng `c3de5245` – Severity-Level im Doctor, Refund-Debit auf „info“ (27.05.2026)
- Sarath `ba2f9456` – Duplicate- & Anomaly-Erkennung; `838596a5`/`47a2795d` – Doctor-UX/Suche/Pagination
- Pratap `d04c7429` – Healthcheck/Triage-Workspace
- coffmant `bee64418` + `911d094c` – Reconciliation-API + Workflow-Seite
- TuDatTr `6fe7cda1` – Negative-Balance-/Debit-Entry-Regeln entfernt (Design-Diskussion)

**Empfehlung:** Severity + Duplicate/Anomaly passt zu unserer Transparenz-Haltung. Reconciliation ist read-only später interessant; keine automatischen Journal-Änderungen.

### P2-07 Editor-Sicherheit: Git-Historie, Verschlüsselung

**d093w1z** `989cee37` – optionale Git-Versionshistorie für das Journal (lokales Repo, Credentials, OAuth; 23 Dateien). **Empfehlung:** Nur lokale Git-History ohne Cloud/OAuth, als Ergänzung zum Backup-Pruning.

**gvijayakumar** `c46598ad` + 13 Folgecommits (Feb.–März 2026) – Verschlüsselung at rest mit In-App-Passwort, Unlock-Gate, Config-Handling. **Empfehlung:** P3, nur wenn wir den Server exponiert betreiben; ältere Basis, Konfliktrisiko.

### P2-08 PWA/Mobile, MCP/AI (optional)

- Sarath `feat/mobile-pwa` (Branch, 229 Commits), nextxm `1f694ec7`, `61a27e00`, `9e5db47b` – PWA/Service-Worker/Mobile-Nav.
- ramkrishna `7c07f2f0`, `bbbb277b`; swapnil `import-mcp` – MCP-Server + AI-REST.
- nextxm `c2348e67` – Command Palette; `ac1600d9`/`6b475487` – NLP-Transaction-Parsing.

**Empfehlung:** Nicht jetzt. PWA ist nice-to-have; MCP/LLM widerspricht unserem lokalen, nachvollziehbaren Standard, solange es keine klare Produktentscheidung gibt.

## 4. Nicht übernehmen (P3)

| Quelle                                                 | Warum nicht                                                                                                                                                                                         |
| ------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| nextxm `develop` als Ganzes (309 Commits)              | Auth/Sessions, Migration-Framework, SQLc, Connect/Protobuf, persistente Job-Queue, PWA, Doctor V3, zahlreiche Produktmodule; zu großer Divergenz- und Wartungsaufwand. Nur isolierte Fixes (P1/P2). |
| Pratap-kute                                            | Deno statt npm, ECharts 6 statt D3/Bulma, umfangreiche UI-Neustrukturierung; passt nicht zu unserem Stack. Feature-Ideen (Healthcheck, Backup/Export, Importer) separat prüfen.                     |
| 1Feng China-spezifisch                                 | OKX/Eastmoney/TTJJ/Yahoo-HK-Scraper, zh-CN-i18n, Receivables, Mortgage, Löschung der Indien-Codebasis.                                                                                              |
| Sarath234 Agent/Indien                                 | SMS-Parser indischer Banken, Gmail/PDF-Statements, Telegram-Bot, Credit-Card-Truth, Axis/ICICI/HDFC-Parser, LLM-Fallback. Widerspricht Local-First und ist fachlich nicht übertragbar.              |
| manishtanwar Indien                                    | Kite/Coin-Integration, Gold-ETF-Tax, Advanced-Expense-UI. (Ausnahmen: Glob-Fix, Import-CLI, kleine UI-Fixes.)                                                                                       |
| matthias Swiss-Reskin / „Ask Claude“                   | Design-Inspiration; Deep-Link zu externem LLM ist keine Kernfunktion. Fix-Batch dagegen übernehmen.                                                                                                 |
| ramkrishna/swapnil MCP                                 | Experimentell; nur bei klarer AI-Strategie.                                                                                                                                                         |
| altanozlu ETF-GraphQL, tonypius Regex-Regeln, rgksugan | Keine neuen eigenen Commits im Fenster; ältere, unmerged Experimente. **Beobachten:** ETF-Metadaten (TER/Look-through) und Regex-Account-Prediction könnten für Roadmap #2/#4 relevant werden.      |
| Bots/Sync-only                                         | Keine eigene Arbeit.                                                                                                                                                                                |

## 5. Vorgeschlagene Reihenfolge

1. **Vor Abschluss des Monatsrückblicks (P1-01 bis P1-05):** Zeitzonen/Datum, hledger-`acost`, `FilterByGlob`, Gehaltsklassifikation, Transfer-Ausschluss. Diese vier verändern die Zahlen des Rückblicks direkt.
2. **Danach (P1-06 bis P1-14):** Yahoo (GBp zuerst), Tooltip, Navbar/Error-Handler, Template-/Import-Fixes, WAL/Backup-Pruning, Sarath-Perf-/Security-Batch, Retirement-Zins, Chart-Margin.
3. **Monatsrückblick-Härtung (P2-01, P2-02):** Sarath-/nextxm-Tests und Randfälle gegen unsere Spezifikation prüfen; FX-/Preis-Frische-Spike.
4. **Später (P2-03 bis P2-08):** Projektion, Rebalancing, Import-Framework, Doctor, Git-Historie; Verschlüsselung/PWA/MCP nur bei Produktentscheidung.

## 6. Hinweise zum Übernehmen

- **Cherry-Pick:** `git fetch https://github.com/<fork>.git <sha>` und `git cherry-pick -x <sha>`. `-x` dokumentiert die Herkunft; Paisa ist AGPL-3.0, unser Fork steht unter derselben Lizenz.
- **Keine Merge-Commits** übernehmen; immer die eigentlichen Feature-/Fix-Commits.
- **Branch-only-Arbeit** (z. B. `matthias`, `stan-ely`, `coffmant`, `d093w1z`, `Sarath`-Feature-Branches) per SHA oder Branch fetchen. GitHub erlaubt `git fetch <url> <sha>` für erreichbare Commits.
- **Konflikte erwarten:** Unser Baum ist gegenüber Juni/Juli 2026 deutlich weiter (Svelte 5, Bulma 1, Monatsrückblick, Deutschland/EU). Kleine Patches manuell portieren; Tests aus dem Fork mitnehmen und an unsere Fixtures anpassen.
- **Sarath `6b730f51`:** unbedingt den Endstand inkl. `fdb5c1eb` nehmen; der Zwischenstand hat einen Compile-/Logikfehler.
- **nextxm:** viele Commits sind rebast/dupliziert. Nicht den Branch mergen; einzelne commits über `git show` prüfen. Die Struktur (`dcPricesTree`, Rate-Cache) existiert bei uns nicht.
- **1Feng:** Der Master enthält viele Merge- und China-Commits; nur die genannten Einzelcommits picken.
- **Verifikation:** `go test ./...`, `npm run check`, `npm run test:prepush`, `npm run test:regression`; bei Preis-/Datumsänderungen zusätzlich die Deutschland/EU-Fixtures neu prüfen.

## 7. Kompakter Commit-Index (P1)

| Thema                  | Fork                 | Commit                                                                                                                              |
| ---------------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Zeitzonen/Datum        | matthiasdebernardini | [`ef719408`](https://github.com/matthiasdebernardini/paisa/commit/ef719408)                                                         |
| Monatsverschiebung     | panuozzo77           | [`67a2fa8a`](https://github.com/panuozzo77/paisa/commit/67a2fa8a)                                                                   |
| hledger `acost`        | nightconcept         | [`40743ba1`](https://github.com/nightconcept/paisa/commit/40743ba1)                                                                 |
| `FilterByGlob` OR/AND  | manishtanwar         | [`122f9eb3`](https://github.com/manishtanwar/paisa-app/commit/122f9eb3)                                                             |
| `FilterByGlob` Prefix  | Sarath234            | [`b5a3d091`](https://github.com/Sarath234/paisa/commit/b5a3d091)                                                                    |
| Payroll/Gehalt         | 1Feng                | [`df35cad3`](https://github.com/1Feng/paisa/commit/df35cad3)                                                                        |
| Salary-Accounts-Config | 1Feng                | [`b31b02bb`](https://github.com/1Feng/paisa/commit/b31b02bb)                                                                        |
| Interne Transfers      | 1Feng                | [`80b0ae5f`](https://github.com/1Feng/paisa/commit/80b0ae5f)                                                                        |
| Yahoo Crumb            | stan-ely             | [`25859f7d`](https://github.com/stan-ely/paisa/commit/25859f7d)                                                                     |
| Yahoo GBp              | mrkstwrt             | [`7f4bd850`](https://github.com/mrkstwrt/paisa-source/commit/7f4bd850)                                                              |
| Tooltip                | bellussi1            | [`5ad11187`](https://github.com/bellussi1/paisa/commit/5ad11187), [`4ad755ff`](https://github.com/bellussi1/paisa/commit/4ad755ff)  |
| Navbar-Crash           | manishtanwar         | [`4295944c`](https://github.com/manishtanwar/paisa-app/commit/4295944c)                                                             |
| Error-Handler          | manishtanwar         | [`0220739b`](https://github.com/manishtanwar/paisa-app/commit/0220739b)                                                             |
| Template-Liste         | 1Feng                | [`0ca8d474`](https://github.com/1Feng/paisa/commit/0ca8d474)                                                                        |
| Nur `.handlebars`      | 1Feng                | [`3ea8aab0`](https://github.com/1Feng/paisa/commit/3ea8aab0)                                                                        |
| Amount-Alignment       | 1Feng                | [`63b10690`](https://github.com/1Feng/paisa/commit/63b10690)                                                                        |
| `negate` Dezimalen     | St-Gonne             | [`f046d740`](https://github.com/St-Gonne/paisa/commit/f046d740)                                                                     |
| SQLite WAL             | Sarath234            | [`22abf928`](https://github.com/Sarath234/paisa/commit/22abf928)                                                                    |
| Backup-Pruning         | Sarath234            | [`43b52794`](https://github.com/Sarath234/paisa/commit/43b52794)                                                                    |
| Perf/Security          | Sarath234            | [`6b730f51`](https://github.com/Sarath234/paisa/commit/6b730f51) + [`fdb5c1eb`](https://github.com/Sarath234/paisa/commit/fdb5c1eb) |
| Retirement-Zins        | sean0407             | [`6b12ddf0`](https://github.com/sean0407/paisa/commit/6b12ddf0)                                                                     |
| Gain-Margin            | 1Feng                | [`4de04c31`](https://github.com/1Feng/paisa/commit/4de04c31)                                                                        |
| Import-Dezimalen       | matthiasdebernardini | [`7c89e0be`](https://github.com/matthiasdebernardini/paisa/commit/7c89e0be)                                                         |

## 8. Offene Punkte für uns

- **Saldo-/Gehaltsdefinition:** Wollen wir `salary_income_accounts` als Config (1Feng) oder eine breitere Heuristik (Sarath: „salary“ im Kontonamen)? Die Config ist expliziter und testbar; die Heuristik ist bequemer, aber fehleranfälliger. Empfehlung: Config, Default `Income:Salary` + `Income:Payroll`.
- **Transfer-Globs:** Welche Konten sind bei unseren Nutzern typische interne Transfers? Ohne Defaults (`transfer_accounts: []`) bleibt das Verhalten unverändert; wir brauchen eine Dokumentation mit Beispielen (`Assets:Tagesgeld`, `Assets:Depot:Verrechnung`).
- **Yahoo-Crumb:** Ist der zusätzliche Cookie-/Crumb-Tanz in unserer Umgebung nötig, oder reicht der GBp-Fix? Vor der Implementierung einen kleinen Netzwerktest mit LSE-/US-Tickern machen.
- **Monatsrückblick-Insights:** Welche Sarath-Insights sind mit unserer TWR-/Frische-Spezifikation vereinbar? Nicht einfach die UI übernehmen; zuerst die Datenregeln abgleichen.
- **Rebalancing/Projektion:** Beide sind P2, aber sie konkurrieren um dieselbe Allocation-/Goals-UI. Erst nach dem Monatsrückblick entscheiden.

## 9. Quellen

- GitHub-Forks: <https://github.com/ananthakumaran/paisa/forks>
- Upstream-Stand: `18ecd7d` (21.10.2026)
- Unser Stand: `f738e0c` (25.10.2026) + laufende Monatsrückblick-Arbeit
- Einzelcommits sind in den Tabellen verlinkt.
