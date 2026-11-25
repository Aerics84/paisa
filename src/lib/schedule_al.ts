import * as d3 from "d3";
import _ from "lodash";
import { get } from "svelte/store";
import { t } from "./i18n";
import { formatCurrency, type ScheduleALEntry } from "./utils";

export function renderBreakdowns(scheduleALEntries: ScheduleALEntry[]) {
  const i18n = get(t);
  const tbody = d3.select(".d3-schedule-al");
  const trs = tbody.selectAll("tr").data(
    scheduleALEntries.concat([
      {
        section: { code: "", section: "", details: i18n("components.scheduleAL.total") },
        amount: _.sumBy(scheduleALEntries, (s) => s.amount)
      }
    ])
  );

  trs.exit().remove();
  trs
    .enter()
    .append("tr")
    .merge(trs as any)
    .html((s) => {
      return `
       <td>${s.section.code}</td>
       <td>${s.section.section}</td>
       <td class="${s.section.code == "" ? "has-text-weight-bold" : ""}">${s.section.details}</td>
       <td class='has-text-right has-text-weight-bold'>${formatCurrency(s.amount)}</td>
      `;
    });
}
