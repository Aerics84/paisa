import { beforeEach, describe, expect, test } from "bun:test";
import dayjs from "dayjs";

import { estimateTargetDate, fallbackForecast, forecastRetirement } from "./goals";
import { setNow, type Point } from "./utils";

function point(date: string, value: number): Point {
  return { date: dayjs(date), value };
}

function createARIMA(predictions: number[]) {
  return class MockARIMA {
    train(_values: number[]) {
      return {
        predict: (_count: number) => [predictions.slice(), predictions.map(() => 0)]
      };
    }
  };
}

describe("retirement forecast helpers", () => {
  beforeEach(() => {
    setNow(dayjs("2026-04-28"));
  });

  const points = [point("2026-03-01", 80), point("2026-04-01", 90)];

  test("keeps ARIMA as the primary forecast path", () => {
    const result = forecastRetirement(points, 100, 90, 5, createARIMA([95, 101]) as never);

    expect(result.forecastMode).toBe("arima");
    expect(result.predictionsTimeline).toHaveLength(2);
    expect(result.predictionsTimeline.at(-1)?.date.format("YYYY-MM-DD")).toBe("2026-04-03");
  });

  test("uses fallback when ARIMA returns no predictions and target is still open", () => {
    const result = forecastRetirement(points, 100, 90, 5, createARIMA([]) as never);

    expect(result.forecastMode).toBe("fallback");
    expect(result.predictionsTimeline.length).toBeGreaterThan(0);
    expect(result.predictionsTimeline.at(-1)?.date.format("YYYY-MM-DD")).toBe("2026-06-01");
  });

  test("returns no forecast when the target is already achieved", () => {
    const result = forecastRetirement(points, 100, 100, 5, createARIMA([101]) as never);

    expect(result).toEqual({
      forecastMode: "none",
      predictionsTimeline: []
    });
  });

  test("does not produce a fallback target date for non-positive monthly contributions", () => {
    expect(estimateTargetDate(100, 90, 0, 0)).toBeNull();
    expect(fallbackForecast(100, 90, 0, 0)).toEqual([]);
  });

  test("uses a conservative zero-rate fallback date", () => {
    const targetDate = estimateTargetDate(100, 70, 10, 0);

    expect(targetDate?.format("YYYY-MM-DD")).toBe("2026-07-01");
  });

  test("pulls the target date forward when a positive rate is available", () => {
    const zeroRateDate = estimateTargetDate(1000, 100, 50, 0);
    const positiveRateDate = estimateTargetDate(1000, 100, 50, 12);

    expect(zeroRateDate?.isAfter(positiveRateDate as dayjs.Dayjs)).toBe(true);
  });
});
