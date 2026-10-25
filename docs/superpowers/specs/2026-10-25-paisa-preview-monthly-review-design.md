# Paisa `/preview`: Monatsrückblick – Design

> **Status: Entwurf zur Freigabe.** Dies ist ein Produkt-/UX-Design, noch kein Implementierungsplan.

## Absicht

Die vorhandene `/preview`-Oberfläche soll zu einem verständlichen persönlichen Finanzrückblick ausgebaut werden. Beim Öffnen soll eine Person schnell sehen können, wie der ausgewählte Monat finanziell verlief, wohin gebuchte Geldflüsse gingen und wie sich die gehaltenen Anlagen entwickelten.

Die elf Reddit-Screenshots dienen als Inspiration, nicht als zu kopierende Oberfläche. Paisas lokaler, Ledger-basierter Ansatz bleibt die Quelle der Wahrheit. Es entsteht weder ein zweites paralleles Dashboard noch eine Trading-Oberfläche.

## Ausgangslage und Problem

- Das klassische Dashboard zeigt aktuelle Vermögens- und Kontostände sowie weitere Einzelbereiche, aber keine zusammenhängende Periodenauswertung.
- `cash_flow/monthly` zeigt aggregierte Monatsbalken und Checking-Balance; die Aussage, wie der Monat insgesamt verlief, ist nicht unmittelbar erkennbar.
- `cash_flow/yearly` zeigt bereits einen nützlichen Konten-Sankey, aber nur jahresweise.
- `expense/monthly` und `expense/yearly` erklären Konsumausgaben, jedoch nicht gemeinsam mit Investitionsbewegungen und Wertentwicklung.
- Die Asset-Ansichten enthalten Werte und Performance, liegen aber getrennt von der Haushaltsauswertung.
- `/preview` enthält bereits Vermögensverlauf, Ausgabenkategorien und Vergleiche, Net-Worth-Treiber, Insights und Goals. Der Prototyp ist jedoch teilweise statisch oder missverständlich: nicht alle Module reagieren auf den Monatspicker, „This month“ sieht interaktiv aus, ohne eine Auswahl zu steuern, und `Cash Flow (MTD)` vermischt verschiedene Bewegungen zu einer schwer deutbaren Zahl.

## Nutzer- und Produktziele

1. Eine gewählte Monatsperiode in `/preview` als vollständigen, verständlichen Finanzrückblick zeigen.
2. Den tatsächlichen Weg gebuchter Geldflüsse erklären und zugleich Überschuss/Defizit klar davon unterscheiden.
3. Nettovermögen und Checking-Bestände aktuell sowie ihre Verläufe sichtbar machen; mehrere Checking-Konten berücksichtigen.
4. Die Wertentwicklung tatsächlicher Positionen im ausgewählten Monat zeigen, Käufe und Verkäufe nicht als Rendite werten und Dividenden als Teil der Gesamtrendite berücksichtigen.
5. Bestehende Insights und Goal Progress erhalten und Goal Progress neben Insights platzieren.
6. Moderne, visuell klare Hierarchie statt mehrerer redundanter KPI-Zeilen schaffen.
7. Unvollständige Daten als unvollständig behandeln; nie fehlende Preise oder Buchungen als 0 oder als sichere Aussage darstellen.

## Abgestimmte Informationshierarchie

### 1. Aktueller Status – unabhängig vom gewählten Monat

Der obere Bereich bleibt ein Live-Snapshot:

- aktuelles Nettovermögen mit einem gut lesbaren, dünn gezeichneten Verlauf;
- aktueller Gesamt-Checking-Bestand über alle Checking-Konten, ergänzt um Verlauf pro Konto;
- kein Recent-Transactions-Modul im neuen Rückblick.

Der Monats-Picker verändert diese Live-Werte nicht.

### 2. Rückblick auf genau einen ausgewählten Monat

Der globale Monatspicker steuert alle periodenspezifischen Rückblick-Module. Die Oberflächenbeschriftung nennt den ausgewählten Monat; es gibt keine zusätzlichen, dekorativen „This month“-Dropdowns. Ist der ausgewählte Monat der aktuelle Monat, zeigt die Rückschau Month-to-date und kennzeichnet den Monat als laufend; historische Monate umfassen den vollständigen Kalendermonat.

Im laufenden Monat reichen Ledger-Werte nur bis zu den tatsächlich gebuchten Postings. Anlagewerte reichen bis zum letzten ausreichend frischen Kurs; dieser Kursstand wird separat kenntlich gemacht. Zukünftige Tage werden nie als Nullwerte ergänzt.

Die Rückschau enthält:

- Nettoeinkommen nach Steuern mit einem Verlauf über die letzten drei Monate;
- einen monatlichen Sankey aus tatsächlichen Ledger-Buchungen, der Konten und Ziele der Geldflüsse zeigt: unter anderem Steuerkonten, Ausgabenkategorien, Anlagekonten und Verbindlichkeiten, soweit Buchungen diese Wege belegen;
- Überschuss/Defizit als separate Kennzahl, nicht als künstlichen Sankey-Zielknoten und nicht als Checking-Balance-Änderung;
- ausgewählte Ausgabenkategorien innerhalb des Geldflusses statt einer zweiten, redundanten Gesamtausgaben-Kachel;
- Anlagebeiträge getrennt von Anlageperformance;
- die besten und schwächsten tatsächlichen Positionen für den ausgewählten Monat, sofern die Performance verlässlich berechenbar ist;
- den bestehenden Net-Worth-Drivers-Gedanken innerhalb des Investment-Bereichs: Beiträge und Marktbewegung bleiben sichtbar, werden aber nicht als ein und dieselbe Kennzahl dargestellt;
- bestehende, auf den ausgewählten Monat bezogene Insights und aktueller Goal Progress nebeneinander. Goals bleiben ein aktueller Stand und werden beim Wechsel des Rückblick-Monats nicht rückdatiert; sie sind als aktuell zu kennzeichnen.

Die Jahresansichten, insbesondere der bestehende Jahres-Sankey und die Investment-Jahreskarten, bleiben separate Ansichten und werden nicht in die Monatsrückschau kopiert.

## Kennzahl- und Datenregeln

### Überschuss/Defizit

- Begriff in der Oberfläche: **Surplus / Deficit** (kein Fachbegriff „Ledger surplus“).
- Formel: `net_income - net_expense`.
- `net_income` folgt Paisas bestehender Definition (`gross_salary_income + gross_other_income - net_tax`).
- `net_expense` schließt `Expenses:Tax` aus, weil Steuern bereits im Nettoeinkommen abgezogen werden.
- Die UI erklärt: rechnerischer Wert aus erfassten Buchungen, nicht Änderung des Checking-Bestands und keine vollständige Vermögensüberleitung.
- Investitionsbeiträge und Tilgung werden nicht ein zweites Mal vom Überschuss abgezogen.

### Ledger-Flussgrafik

- Sie zeigt Kontenflüsse aus den tatsächlich gebuchten Transaktionen des ausgewählten Monats; der bestehende Jahres-Sankey ist die Referenz für Hierarchie und Interaktion.
- Die Sankey-Datenlogik bleibt fachlich getrennt von der Anlageperformance: Sie gruppiert und paart Soll-/Haben-Postings pro Ledger-Transaktion und zeigt Kontenpfade, keine Rendite.
- Hierarchische Kontengruppen dürfen keine Bewegungen doppelt zählen. Reconciliation-Tests müssen pro Transaktion die Eingangspostings, Ausgangspostings und visualisierten Flüsse nachvollziehbar abgleichen; Summen über mehrere Hierarchieebenen dürfen nicht naiv addiert werden.
- Interne Transfers zwischen Checking-Konten bleiben Asset-zu-Asset-Flüsse, nicht Einkommen oder Konsum. Investitionskäufe bleiben Anlageflüsse, nicht Expenses. Tilgung auf Liability-Konten und Zinsaufwand auf Expense-Konten bleiben getrennte Flussarten.
- Aus dem Überschuss wird kein erfundener „übriger Cash“-Fluss abgeleitet.
- Gruppierung/Tiefe soll Details verständlich halten, ohne die zugrunde liegenden Buchungen umzudeuten.

### Monatsperformance tatsächlicher Positionen

- Ranking bezieht sich auf den ausgewählten Monat, nicht auf die gesamte Haltedauer.
- Käufe und Verkäufe sind Cashflows und dürfen nicht als Performance gelten.
- Dividenden zählen zur Gesamtrendite und werden als eigener Bestandteil sichtbar gehalten.
- Dividenden dürfen nur einer Position zugeordnet werden, wenn die Ledger-Buchung diese Quelle verlässlich identifiziert. Nicht eindeutig zuordenbare Dividenden dürfen nicht willkürlich in die Rendite einer Position einfließen.
- Gezeigt werden bis zu drei Gewinner und drei schwächste Positionen als knappe Dashboard-Übersicht; die vollständige Analyse bleibt in den vorhandenen Investment-/Gain-Details.
- Alle Positionen in derselben Rangliste müssen dieselbe ausgewählte Monatsperiode, dieselbe konfigurierte Basiswährung, dieselbe Renditemethode und vergleichbare Kursabdeckung verwenden. Nicht vergleichbare Positionen werden nicht stillschweigend in das Ranking gemischt.
- Lifetime-XIRR darf nicht als Monatsrendite ausgegeben werden.
- Positionen werden nur gerankt, wenn historische Bewertungen und zugehörige Ledger-Flows eine belastbare Monatsberechnung erlauben. Gibt es insgesamt zu wenig verlässliche Daten, zeigt das Modul einen einzigen knappen „not available“-Zustand. Keine Liste jeder ausgeschlossenen Position und keine stillschweigende 0-Rendite.
- Bei laufenden Monaten gilt die Rangliste Month-to-date bis zum letzten ausreichenden Preisdatum; der Datenstand wird am Investment-Modul ausgewiesen.
- Vor Implementierung müssen Return-Formel, Behandlung von Cashflows/Dividenden und Kursfrische-Regeln anhand gespeicherter Preisverläufe und Tests validiert werden. Die Berechnung darf nicht allein aus einer Kursänderung des Wertpapiers bestehen.
- Anlageperformance und Sankey verwenden ausdrücklich unterschiedliche Berechnungsmodelle; Ein-/Auszahlungen und Dividenden in der Renditeberechnung dürfen nicht aus Sankey-Linkbreiten abgeleitet werden.

### Validierte Monatsperformance-Methode (Spike-Ergebnis, 2026-10-25)

Der technische Spike ist **GO**: Die folgende Methode ist mit den vorhandenen Bewertungen und Ledger-Flows belastbar umsetzbar.

- **Renditemethode:** täglich verkettete, zeitgewichtete Gesamtrendite (TWR) auf Basis von Bewertungen in der konfigurierten Basiswährung. Der Startwert ist der letzte Kurs **vor** dem Monatsersten (Vortagesschluss), damit auch die Kursbewegung des ersten Kalendertags Teil der Rendite ist. Für jeden bewerteten Tag mit vorherigem Kapitaleinsatz gilt `r_t = (V_t - Käufe_t + Verkäufe_t + Dividenden_t) / V_{t-1} - 1`; die Monatsrendite ist `(Π(1 + r_t)) - 1`. Tage ohne Bestand und ohne Flow haben kein Kapitalrisiko und erzeugen keine Teilrendite.
- **Euro-Gewinn/-Verlust:** `V_Ende - V_Start - Nettoeinzahlungen + Nettoauszahlungen + Dividenden`, wobei `V_Start` der Vortagesschluss vor dem Monatsersten ist. Käufe und Verkäufe sind Cashflows und keine Rendite. Eine vollständig verkaufte Position bleibt bis zum Verkaufstag bewertbar.
- **Investment-Treiber:** "Market movement" ist die kursbedingte Bewegung der gerankten Positionen **ohne** Dividenden; Dividenden werden als eigener Bestandteil ausgewiesen. Netto-Beiträge sind vorzeichenbehaftet über alle Nicht-Checking-Assets (Käufe minus Verkäufe und Umbuchungen) und damit nicht dasselbe wie die Marktbewegung.
- **Modified Dietz wurde verworfen:** Bei Kauf aus Nullbestand und bei Verkäufen innerhalb des Monats weicht die Näherung material ab (Spike: 19,4 % statt 10 % bzw. 17,8 % statt 20 %), weil sie den Zeitpunkt der Flows nur gewichtet statt täglich zu bewerten. Lifetime-XIRR bleibt als Monatswert ausgeschlossen.
- **Dividendenzuordnung:** `Income:Dividend:<Rest>` wird nur dann einer Position zugeordnet, wenn das letzte Segment von `<Rest>` eindeutig dem letzten Segment genau einer gehaltenen Position entspricht. Nicht eindeutige oder nicht zuordenbare Dividenden bleiben in der Gesamtdividende sichtbar, fließen aber in keine Positionsrendite ein.
- **Kursfrische:** Es gilt der letzte gespeicherte Kurs am oder vor dem Bewertungstag; der tatsächliche Kursstichtag wird mitgeführt. Ein Carry-forward über Wochenenden und Börsenfeiertage ist zulässig, solange der Kurs höchstens **7 Kalendertage** alt ist; ältere Kurse machen die Position nicht rankbar. Für die Rangliste sind mindestens **zwei verschiedene Kursstichtage mit Kapitaleinsatz** erforderlich. Ein einzelner durchgezogener Kurs ergibt keine 0-%-Rendite, sondern „nicht verfügbar“.
- **Aktien-Splits/Konsolidierungen:** Transaktionen, die nur Mengen ohne Zahlungsfluss ändern, werden nicht als Rendite interpretiert. Betroffene Positionen werden im ausgewählten Monat nicht gerankt, statt eine verzerrte oder 0-%-Rendite zu zeigen.
- **Abdeckungsschwelle:** Die Rangliste erscheint nur, wenn mindestens **zwei Positionen** belastbar berechenbar sind und diese mindestens **50 % des Endmarktwerts** aller Kandidatenpositionen abdecken. Eine Position ohne verwendbaren Endkurs oder ein Konto mit mehreren bewerteten Commodities macht die Abdeckung unvollständig; dann erscheint genau ein kompakter „not available“-Zustand, statt eine Teilmenge als vollständig auszugeben.
- **Laufender Monat:** Die Bewertungsreihe endet am letzten ausreichend frischen Kursstichtag; dieser wird als `price_as_of` am Investment-Modul ausgewiesen. Zukünftige Tage werden nicht als Null ergänzt.
- **Datumsgrenzen:** Monatsfilter vergleichen den im Ledger gespeicherten Kalendertag (`YYYY-MM-DD`), nicht den Zeitstempel mit Zeitzonen-Offset. Ein am Monatsersten gebuchter Posten darf nicht durch einen Offset-Unterschied zwischen Ledger-Sync und konfigurierter Zeitzone verschwinden.
- **Checking-Historie und Monatspicker:** Die Checking-Historie enthält zwölf aufeinanderfolgende Kalendermonate (leere Monate tragen den letzten Stand). Der Picker endet nie vor dem aktuellen Monat, damit der laufende Monat auch ohne bisherige Buchung als Month-to-date ausgewählt werden kann.

### Trends und fehlende Daten

- Veränderungen werden als kompakte Pfeile und Zahlen angezeigt; kein wiederholter Text wie „positive trend“ oder „compared with April 2026“.
- Für einen laufenden Monat ist der aktuelle Einkommenspunkt als MTD/ongoing erkennbar und visuell von vollständigen Monaten unterschieden. Er erhält keinen roten/grünen Delta-Vergleich zu einem vollständigen Monat, der allein durch die kürzere Dauer einen Rückgang suggeriert.
- Farben geben nur dann eine positive/negative Wertung, wenn sie fachlich sinnvoll ist. Ein steigender Steuerbetrag oder Checking-Bestand wird nicht automatisch als gut oder schlecht bewertet.
- „Keine Buchungen“, „laufende/fehlende Historie“ und echter negativer Überschuss bleiben unterscheidbar.
- Historische Preisabdeckung muss erkennbar sein; stale Preise dürfen nicht als aktuelle Monatsbewertung ausgegeben werden.
- Preisalter muss nach Commodity-/Kurstyp bewertet werden; ein Wochenende oder Börsenfeiertag ist nicht automatisch eine Kurslücke, ein beliebig alter letzter Kurs aber auch kein aktueller Monatsendkurs.

### Checking, Überschuss und Geldflüsse

- Veränderungen einzelner Checking-Konten, Gesamtbestand, gebuchte Geldflüsse und Surplus/Deficit sind unterschiedliche Größen und bleiben getrennt.
- Umbuchungen zwischen eigenen Checking-Konten dürfen den Haushaltsüberschuss nicht verändern.
- Monatsüberschuss ist kein Cash-Rest und wird nicht durch Investitionsbeiträge oder Tilgungen „abgestimmt“.
- Der Einkommensverlauf und Sankey müssen dieselben Kalender-Monatsgrenzen verwenden. Das Nettoeinkommen folgt der V1-Formel und ist erklärbar; der Sankey zeigt die zugrunde liegenden Buchungen.

## Nichtziele

- Keine neue Dashboard-Route oder ein paralleles Finanzprodukt: `/preview` wird ausgebaut.
- Keine Überarbeitung oder Vermischung der getrennten Jahres-, Ausgaben-, Asset- und Steueransichten.
- Keine Recent Transactions auf dem Monatsrückblick.
- Keine separate Steuerkachel oder neue Steuerverlinkung; `More → Tax` bleibt wie bisher. Steuerberechnungen, Vorabpauschale und weitere Steuerfälle bleiben ein separates Projekt.
- Keine obligatorische Bank-/Broker-Anbindung, keine Änderung des Journals, keine automatischen Buchungsänderungen.
- Keine Kauf-/Verkaufssignale, RSI/MACD-Scanner oder Anlageempfehlungen.
- Keine Rankings aus unbereinigten Kursänderungen und keine Renditen aus veralteten oder unvollständigen Preisreihen.

## Umsetzungs- und Qualitätskriterien

1. Der Monatspicker aktualisiert alle Periodenmodule konsistent; Live-Nettovermögen und aktuelle Checking-Stände bleiben davon unabhängig.
2. Der Sankey basiert auf den tatsächlichen Buchungen des ausgewählten Monats und lässt sich mit der Jahresansicht fachlich vergleichen.
3. Überschuss/Defizit entspricht der dokumentierten Formel und wird nicht mit Kontostandsänderung oder Anlageperformance verwechselt.
4. Checking-Verläufe unterstützen mehrere Konten; Net-Worth- und Checking-Charts bleiben bei responsiver Darstellung dünn und lesbar.
5. Nettoeinkommensverlauf nutzt den ausgewählten Monat plus zwei Vormonate; fehlende Monate erscheinen nicht als 0.
6. Monatsperformance pro Position ist gegen Käufe, Verkäufe, Dividenden und Preisabdeckung getestet. Lifetime-XIRR wird nicht als Monatswert verwendet.
7. Unverbundene/unerklärte Kontrollen wie das bisherige „This month“-Chevron werden entfernt oder tatsächlich an die globale Monatsauswahl gebunden.
8. Goal Progress und bestehende Insights bleiben sichtbar; Tax- und Jahresberichte bleiben unverändert.
9. Es gibt keine Demo-Werte oder stillen Erfolgszustände in der produktiven Vorschau.

## Bekannte Implementierungsfragen für den Plan

- `GetGain()` liefert aktuell pro Konto gegenwärtigen Gain/Loss und Lifetime-XIRR; `GetAccountGain()` liefert pro Konto eine tägliche `NetworthTimeline`. Die Übersicht benötigt eine periodenspezifische, flow-adjustierte Performance-Ausgabe.
- `GetUnitPrice()` verwendet den letzten gespeicherten Kurs am oder vor dem Bewertungsdatum ohne maximale Kursalter-Prüfung. Die `Price`-Entität enthält Zeitstempel und Commodity-Felder, aber keinen expliziten Freshness-Status.
- `Income:Dividend:*` existiert als Buchungskonvention, wird aber nicht aktuell der per-Positionen-Gain-Berechnung zugeordnet. Diese Zuordnung muss anhand realer Kontonamen und Testfälle bestätigt werden, bevor Dividenden in ein Positionsranking einfließen.
- Der Planer soll deshalb die Berechnungsketten getrennt inventarisieren und dann einen technischen Spike mit repräsentativen Ledger-/Preisdaten ausführen: Sankey-Paarung und Summenabgleich einerseits; Monats-Total-Return mit Käufen, Verkäufen, Dividenden, FX und Kurslücken andererseits. UI-Änderungen beginnen erst, wenn beide Modelle getestet sind.
- Der Spike muss eine konkrete Renditemethode, Kursfrische-/Abdeckungsregeln und eine Dividendenzuordnung liefern. Wenn das vereinbarte Top/Flop-Ranking nicht zuverlässig gebaut werden kann, muss die Produktentscheidung vor Implementierungsbeginn zurück an die Nutzerin — nicht stillschweigend auf Lifetime-XIRR oder Preisänderung ausweichen.

## Fehlende-Daten-Zustände

| Zustand                                               | Darstellung                                                                                          |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Vollständiger Monat ohne Buchungen                    | No recorded activity                                                                                 |
| Laufender Monat                                       | Ausgewählter Monat ist laufend; nur tatsächlich gebuchte Werte                                       |
| Monat ohne passende Postings                          | No recorded activity; daraus wird keine Vollständigkeit der Ledger-Historie abgeleitet               |
| Import explizit als unvollständig diagnostiziert      | Import-/Datenhinweis nur dann anzeigen, wenn Paisa dafür ein tatsächliches Diagnosesignal hat        |
| Fehlende oder stale Monatsendkurse                    | Monatsperformance nicht verfügbar                                                                    |
| Kurs-/Preisabdeckung insgesamt nicht ausreichend      | Ein kompakter Not-available-Zustand für die Rangliste; keine Liste aller ausgeschlossenen Positionen |
| Tatsächlicher negativer Überschuss                    | Negativer Wert, kein Datenfehler                                                                     |
| Positive/negative Wertänderung mit unklarer Bedeutung | Richtung und Betrag neutral anzeigen, nicht pauschal als gut/schlecht werten                         |

Die aktuelle Anwendung kennt keinen allgemeinen Beleg dafür, dass ein Ledger für einen Zeitraum vollständig ist. Sie darf deshalb aus fehlenden Postings weder auf vollständige Erfassung noch auf einen Importfehler schließen.

## Umsetzungsreihenfolge

1. Bestandsaufnahme der bestehenden `/preview`-Komponenten, API-Daten, Periodenfilter, Jahres-Sankey und Preisverläufe.
2. Technischer Spike und Tests für die zwei getrennten Modelle: Ledger-Sankey mit Transaktions-/Summenabgleich sowie Monatsperformance mit Zahlungsströmen, Dividenden und Kursabdeckung.
3. Datenmodelle/API und Regressionstests erst nach Freigabe der Formeln, Zustände und Abdeckungsregeln.
4. `/preview`-Integration: konsistenter Monatspicker, aktuelle Live-Kennzahlen separat, echte Monats-Sankey-/Performance-Module und korrigierte Diagrammstyles.
5. Integrationstest mit historischen Monaten, mehreren Checking-Konten, Dividenden, Trades, Kurslücken und leerer/teilweiser Historie; danach vollständige Regression.

Bei einem nicht bestandenen Spike stoppt die Umsetzung vor dem UI-Schritt; die offene Produktentscheidung wird mit Befunden zurück an die Nutzerin gegeben.

## Bewusste Produktentscheidungen

- Bestehendes `/preview` wird als Grundlage ausgebaut; die klassische Startseite wird in diesem Arbeitspaket nicht automatisch ersetzt.
- Die Übersicht ist eine Monatsrückschau; Jahresrückblicke bleiben in den bestehenden Jahresansichten.
- Steuerinformationen bleiben aus der neuen Übersicht heraus, weil `More → Tax` bereits existiert.
- Die Preis-/Performance-Rangliste ist ein Pflichtbestandteil der vollständigen Lieferung, aber nur mit validierter Methode und expliziter Datenverfügbarkeit.
