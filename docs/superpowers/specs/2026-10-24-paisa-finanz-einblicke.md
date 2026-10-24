# Paisa: Finanz-Einblicke – Design

## Absicht

Die in den elf Reddit-Screenshots gezeigten Ideen sollen auf Paisas lokale, Ledger-basierte Arbeitsweise übertragen werden. Der Schwerpunkt liegt nicht auf Trading-Signalen, sondern darauf, verständlich zu zeigen, wie Einkommen, Ausgaben, Sparen und Vermögensentwicklung zusammenhängen.

## Ausgangslage

- Paisa hat bereits Vermögensverläufe, XIRR, Anlagequoten, Gewinnansichten, jährliche Investmentkarten, Cashflow- und Income-Statement-Berichte sowie eine alternative, datenverbundene Oberfläche unter `/preview`.
- Die bestehende jährliche `SavingsRate` ist `net_investment / net_income`. Sie beschreibt damit die Investitionsquote, nicht den gesamten Überschuss aus Nettoeinkommen abzüglich Ausgaben.
- Die Investment-Analyse nach Wertpapierart, Rating und Branche ist vorhanden, aber als experimentell dokumentiert. Benchmarkvergleich, TWROR, gewichtete TER, Länder-/Unternehmensgrößen-Look-through und Drawdown-Analyse sind im geprüften Funktionsumfang nicht erkennbar.
- Der deutsche Kapitalertragsbericht deckt bereits Teile der Besteuerung ab, ist aber laut Dokumentation kein vollständiges Steuermodell. Die Vorabpauschale wird dort nicht aufgeführt.

## Ziele

1. Gesamt-Sparquote und Investitionsquote klar unterscheiden.
2. Die vorhandene Jahresansicht als erste, kleine und überprüfbare Ausbaustufe nutzen.
3. Bestehende `/preview`-Komponenten weiterverwenden, statt ein zweites paralleles Dashboard neu zu bauen.
4. Folgefunktionen für Depotanalyse und Renditevergleich getrennt und in fachlich prüfbaren Schritten planen.
5. Formeln, Datenabdeckung und Grenzen sichtbar machen; fehlende Daten dürfen nicht als Null oder sichere Prognose erscheinen.

## Nichtziele

- Kein verpflichtender Bank-/Broker-Cloud-Sync; lokale Kontoauszüge und Ledger-Dateien bleiben der Standard.
- Keine Kauf-/Verkaufssignale, Aktien-Scanner, RSI-/MACD-Watchlist oder automatische Anlageempfehlungen im Standardprodukt.
- Keine Vorabpauschalen- oder sonstige Steuerberechnung ohne eigenständige fachliche Spezifikation, überprüfbare Quellen und Tests.
- Keine Änderung der Ledger-Dateien oder automatisches Umschreiben von Buchungen durch Analysefunktionen.

## V1: Jahresübersicht „Gesamt sparen vs. investieren“

Die erste Umsetzung ergänzt die vorhandenen Jahreskarten auf der Investment-Seite. Die Berechnung darf nicht davon abhängen, dass Investment-Postings existieren: auch bei Einkommen und Ausgaben ohne Anlagebuchungen müssen Jahreskarten entstehen. Nur ein vollständig leeres Journal liefert keine Karte.

- **Nettoeinkommen:** vorhandene Berechnung `gross_salary_income + gross_other_income - net_tax` weiterverwenden. „Other income“ bleibt in V1 wie bisher definiert; die Anzeige darf es nicht als reines Erwerbseinkommen bezeichnen.
- **Nettoausgaben:** vorhandenes `net_expense`, ohne `Expenses:Tax`; Steuern sind bereits im Nettoeinkommen abgezogen.
- **Gesamter Sparüberschuss:** `net_income - net_expense`.
- **Gesamte Sparquote:** `100 * (net_income - net_expense) / net_income`, nur wenn `net_income > 0`; sonst wird die Quote als nicht verfügbar angezeigt, der absolute Überschuss bleibt sichtbar.
- **Investitionsquote:** die bisherige Kennzahl `100 * net_investment / net_income` beibehalten, aber in der Oberfläche als „Investitionsquote“ bezeichnen.
- Bei `net_income <= 0` werden beide Quoten als nicht verfügbar angezeigt. Negative Überschüsse und Quoten über 100 % werden nicht abgeschnitten.
- Die UI bezeichnet die Kennzahl ausdrücklich als rechnerischen Überschuss aus den erfassten Einkommens- und Ausgabenbuchungen, nicht als Veränderung des Bankguthabens oder vollständige Überleitung zum Vermögenszuwachs.
- `period_complete` beschreibt ausschließlich, ob das Finanzjahr kalendarisch abgeschlossen ist. `has_activity` beschreibt, ob für den Zeitraum relevante Buchungen erfasst sind. Keines der Felder beweist, dass der Ledger alle realen Vorgänge enthält.
- Zeiträume ohne relevante Buchungen werden als „No recorded activity“ dargestellt; ein belegter negativer Überschuss bleibt ein negativer Wert und wird nicht als fehlende Daten behandelt.
- Unvollständige Geschäftsjahre werden als laufendes Jahr/YTD erkennbar gehalten. Kennzahlen beruhen immer auf den im Ledger erfassten Buchungen.
- V1 zerlegt den Überschuss noch nicht in Cashbestand, Tilgung und weitere Vermögensbewegungen. Sie darf deshalb nicht behaupten, diese Verwendungen vollständig zu erklären.
- API-Werte werden nicht vorzeitig gerundet; die vorhandene UI-Formatierung zeigt Quoten mit zwei Nachkommastellen und konfigurierter Locale.

## Roadmap nach V1

Diese Bereiche sind fachlich eigenständige Folgeprojekte und sollen jeweils separat spezifiziert werden:

1. **Geldfluss-Dashboard:** monatliche und jährliche Sicht auf Konsum, Steuern, Investitionen, Tilgung und übrigen Überschuss; vorhandene `/preview`-Ansichten wiederverwenden.
2. **Depot-Positionsübersicht:** Marktwert, Einstand, Gewinn/Verlust, Anteil am Portfolio, XIRR, Dividenden und Datenstand in einer Tabelle; optional manuell gepflegte TER mit gewichteten Kosten.
3. **Renditeanalyse:** zuerst Datenprüfung für historische Bewertungen, externe Zahlungsströme, Kurslücken, Währungen und Bewertungszeitpunkte. Dieser Audit muss einen begründeten Mindest-Abdeckungsgrad festlegen. Bis er erreicht ist, sind TWROR und Drawdown „not available“ statt stillschweigend geschätzte Werte. Benchmarkvergleiche müssen dieselben externen Zahlungsströme berücksichtigen und klar als historische Gegenrechnung gekennzeichnet sein.
4. **Portfolio-Look-through:** Länder- und Unternehmensgrößen-Verteilung samt Herkunft, Aktualität und Abdeckungsgrad; vorhandene Branchen-/Ratinganalyse bleibt als experimentell gekennzeichnet, bis Datenqualität nachgewiesen ist.
5. **Steuererweiterung:** deutsche Vorabpauschale und weitere Steuerfälle nur mit separater Fachspezifikation und Tests.

## Querschnittsanforderungen

- Alle Kennzahlen werden aus dem Ledger und vorhandenen Preisen abgeleitet; keine Analyse überschreibt die Quelle.
- Finanzjahrgrenzen folgen Paisas bestehender `financial_year_starting_month`-Konfiguration.
- Der Nenner und die enthaltenen Einkommens-/Ausgabenbuchungen müssen in der UI erklärbar sein.
- Neue UI-Texte folgen der bestehenden englischen Produktoberfläche; die Referenzdokumentation erklärt die Formeln.
- Fehlende Aktivität, ein laufender Zeitraum und ein tatsächlich negativer Überschuss werden unterscheidbar dargestellt. Paisa behauptet nicht, aus einem Ledger allein fehlende, nie erfasste Buchungen erkennen zu können.
- Fehlende oder unvollständige Historie wird als „not available“/„incomplete“ markiert, nicht als 0 % oder als abgeschlossene Periode.
- Backend-/API-Raten bleiben ungerundet; die UI verwendet die bestehende `formatFloat(value, 2)`-Formatierung und konfigurierte Locale.
- Externe Kurse oder Benchmarks bleiben opt-in und zeigen Quelle sowie Datenstand.
- Eine programmierbare Renditeformel allein ist kein Freigabekriterium: benötigte historische Bewertungen müssen an den relevanten Zahlungsstromzeitpunkten ausreichend vorhanden sein.

## Erfolgskriterien für V1

- Eine Nutzerin kann Gesamt-Sparquote und Investitionsquote unterscheiden und die Formel nachvollziehen.
- Bei null oder negativem Nettoeinkommen entsteht keine Division durch null und keine irreführende Quote.
- Negative Überschüsse bleiben sichtbar; laufende Finanzjahre werden nicht als vollständig dargestellt.
- Einkommen-/Ausgabenbuchungen ohne Investment-Postings erzeugen trotzdem eine Jahreskarte.
- Steuern werden genau einmal berücksichtigt; keine Aktivität, laufende Finanzjahre und belegte Defizite sind unterscheidbar.
- Bestehende Investment-, Dashboard- und Regression-Ansichten bleiben kompatibel.
