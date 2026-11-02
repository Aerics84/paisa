/**
 * The financial year to show: the current one when it is still available,
 * otherwise the latest available. The shared year starts empty and its range
 * changes once a page has loaded its data, so a value outside the range would
 * show an empty year or a blank picker. `years` is ordered latest first.
 */
export function resolveFinancialYear(value: string, years: string[]): string {
  if (years.length === 0) return "";
  return years.includes(value) ? value : years[0];
}
