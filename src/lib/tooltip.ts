const ROW_HEIGHT = 24;
const VERTICAL_SPACING = 32;
const MIN_ROWS = 5;

type Cell = string | string[];
type Row = Cell[];

function renderRows(rows: Row[]): string {
  const trs = rows
    .map((r) => {
      const cells = r
        .map((c) => {
          if (typeof c == "string") {
            return `<td>${c}</td>`;
          } else {
            if (c.length == 3) {
              return `<td class='${c[1]}' colspan='${c[2]}'>${c[0]}</td>`;
            }
            return `<td class='${c[1]}'>${c[0]}</td>`;
          }
        })
        .join("\n");

      return `<tr>${cells}</tr>`;
    })
    .join("\n");
  return `<table class='table is-narrow is-size-7 popup-table'><tbody>${trs}</tbody></table>`;
}

function viewportRows(): number {
  const height = typeof window !== "undefined" && window.innerHeight ? window.innerHeight : 768;
  return Math.max(MIN_ROWS, Math.floor((height - VERTICAL_SPACING) / ROW_HEIGHT));
}

export function tooltip(
  rows: Array<Array<string | string[]>>,
  options: {
    header?: string;
    total?: string;
  } = {}
) {
  // Work on a copy: callers reuse their row arrays for charts.
  const body = rows.map((row) => [...row]);
  const maxRows = viewportRows();

  if (body.length === 0) {
    return renderRows([]);
  }

  const columnCount = body[0].length;
  const totalRow: Cell[] = [];
  const headerRow: Cell[] = [];

  if (options.total) {
    totalRow.push(["Total", "has-text-weight-bold"]);
    totalRow.push([options.total, "has-text-weight-bold has-text-right"]);
    for (let i = 2; i < columnCount; i++) {
      totalRow.unshift("");
    }
  }

  if (options.header) {
    headerRow.push([
      options.header,
      "has-text-weight-bold has-text-centered",
      columnCount.toString()
    ]);
  }

  // Reserve space for the header and the total row, then keep the newest
  // rows and summarise the ones that no longer fit.
  const reserved = (headerRow.length > 0 ? 1 : 0) + (totalRow.length > 0 ? 1 : 0);
  const available = Math.max(1, maxRows - reserved);

  let visible = body;
  let omitted = 0;
  if (body.length > available) {
    omitted = body.length - available;
    visible = body.slice(omitted);
  }

  const finalRows: Row[] = [];
  if (headerRow.length > 0) {
    finalRows.push(headerRow);
  }
  if (omitted > 0) {
    finalRows.push([[`${omitted} earlier rows omitted`, "", columnCount.toString()]]);
  }
  finalRows.push(...visible);
  if (totalRow.length > 0) {
    finalRows.push(totalRow);
  }

  return renderRows(finalRows);
}
