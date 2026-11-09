import type dayjs from "dayjs";

export function shouldShowNoDues(paidDate: dayjs.Dayjs | null | undefined, amountDue: number) {
  return !paidDate && amountDue < 1;
}
