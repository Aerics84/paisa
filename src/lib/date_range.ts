import type dayjs from "dayjs";

export interface DateBounds {
  dateMin: dayjs.Dayjs;
  dateMax: dayjs.Dayjs;
}

/**
 * The earliest and latest dates in `dates`, or null when there are none.
 * Missing values are ignored.
 */
export function dateBounds(dates: (dayjs.Dayjs | null | undefined)[]): DateBounds | null {
  let dateMin: dayjs.Dayjs | undefined;
  let dateMax: dayjs.Dayjs | undefined;

  for (const date of dates) {
    if (!date) continue;
    if (!dateMin || date.isBefore(dateMin)) dateMin = date;
    if (!dateMax || date.isAfter(dateMax)) dateMax = date;
  }

  return dateMin && dateMax ? { dateMin, dateMax } : null;
}
