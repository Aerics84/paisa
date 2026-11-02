import * as d3 from "d3";
import { Delaunay } from "d3";
import type dayjs from "dayjs";
import _ from "lodash";
import tippy from "tippy.js";
import COLORS from "./colors";
import {
  formatCurrency,
  formatCurrencyCrudeWithLocale,
  isMobile,
  now,
  svgUrl,
  tooltip,
  type CurrencyNetworth,
  type Legend,
  type Networth
} from "./utils";

function networth(d: Networth) {
  return d.investmentAmount + d.gainAmount - d.withdrawalAmount;
}

function investment(d: Networth) {
  return d.investmentAmount - d.withdrawalAmount;
}

export interface PeriodStats {
  networth: number;
  investment: number;
  gain: number;
  xirr: number;
}

/**
 * A converted currency without any timeline has no FX rates. Showing $0.00 /
 * 0.00% would contradict the empty chart, so the KPI cards are hidden instead.
 * The default currency keeps its existing zero-value behaviour.
 */
export function hasNetworthKpiData(
  selected: CurrencyNetworth | undefined,
  points: Networth[]
): boolean {
  return !selected || points.length > 0;
}

function xirrOf(flows: { date: dayjs.Dayjs; amount: number }[]): number {
  if (flows.length < 2 || !flows.some((f) => f.amount < 0) || !flows.some((f) => f.amount > 0)) {
    return 0;
  }
  const t0 = flows[0].date;
  const years = flows.map((f) => f.date.diff(t0, "day") / 365);
  const npv = (r: number) =>
    flows.reduce((sum, f, i) => sum + f.amount / Math.pow(1 + r, years[i]), 0);

  // npv is decreasing in r, so bisect
  let lo = -0.9999,
    hi = 1000;
  if (npv(lo) * npv(hi) > 0) {
    return 0;
  }
  for (let i = 0; i < 200; i++) {
    const mid = (lo + hi) / 2;
    if (npv(lo) * npv(mid) <= 0) {
      hi = mid;
    } else {
      lo = mid;
    }
  }
  return ((lo + hi) / 2) * 100;
}

// Stats for the selected period. The balance at the start of the period is
// treated as the initial investment for XIRR.
export function computePeriodStats(points: Networth[], from: dayjs.Dayjs, to: dayjs.Dayjs) {
  const inRange = _.filter(points, (p) => p.date.isSameOrBefore(to) && p.date.isSameOrAfter(from));
  const before = _.findLast(points, (p) => p.date.isBefore(from));
  const end = _.last(inRange);
  if (!end) {
    return { networth: 0, investment: 0, gain: 0, xirr: 0 } as PeriodStats;
  }

  const nw = (p?: Networth) => (p ? networth(p) : 0);
  const ni = (p?: Networth) => (p ? investment(p) : 0);
  const periodInvestment = ni(end) - ni(before);

  const flows = [{ date: before ? before.date : inRange[0].date, amount: -nw(before) }];
  let prev = before;
  _.each(inRange, (p) => {
    flows.push({ date: p.date, amount: -(ni(p) - ni(prev)) });
    prev = p;
  });
  flows.push({ date: end.date, amount: nw(end) });

  return {
    networth: nw(end),
    investment: periodInvestment,
    gain: nw(end) - nw(before) - periodInvestment,
    xirr: xirrOf(flows)
  } as PeriodStats;
}

export function renderNetworth(
  points: Networth[],
  element: Element,
  options: { currencySymbol?: string } = {}
): { destroy: () => void; legends: Legend[] } {
  // The user's locale (e.g. lakh / crore grouping) applies to the default
  // currency only.
  const symbol = options.currencySymbol || "";
  const locale = symbol ? "en-US" : null;
  const fmt = (v: number) => symbol + formatCurrency(v, null, locale);
  const fmtCrude = (v: number) => symbol + formatCurrencyCrudeWithLocale(v, locale);

  const start = _.min(_.map(points, (p) => p.date)),
    end = now();

  const svg = d3.select(element);

  svg.selectAll("*").remove();

  const right = isMobile() ? 10 : 80,
    margin = { top: 15, right: right, bottom: 20, left: 40 },
    width = Math.max(element.parentElement.clientWidth, 800) - margin.left - margin.right,
    height = +svg.attr("height") - margin.top - margin.bottom,
    g = svg.append("g").attr("transform", "translate(" + margin.left + "," + margin.top + ")");

  svg.attr("width", width + margin.left + margin.right);

  const areaKeys = ["gain", "loss"];
  const colors = [COLORS.gain, COLORS.loss];
  const areaScale = d3.scaleOrdinal<string>().domain(areaKeys).range(colors);

  const lineKeys = ["networth", "investment"];
  const lineScale = d3
    .scaleOrdinal<string>()
    .domain(lineKeys)
    .range([COLORS.primary, COLORS.secondary]);

  const positions = _.flatMap(points, (p) => [
    p.gainAmount + p.investmentAmount - p.withdrawalAmount,
    p.investmentAmount - p.withdrawalAmount
  ]);
  positions.push(0);

  const x = d3.scaleTime().range([0, width]).domain([start, end]),
    y = d3.scaleLinear().range([height, 0]).domain(d3.extent(positions)),
    z = d3.scaleOrdinal<string>(colors).domain(areaKeys);

  const area = (y0: number, y1: (d: Networth) => number) =>
    d3
      .area<Networth>()
      .curve(d3.curveMonotoneX)
      .x((d) => x(d.date))
      .y0(y0)
      .y1(y1);

  g.append("g")
    .attr("class", "axis x")
    .attr("transform", "translate(0," + height + ")")
    .call(d3.axisBottom(x));

  if (!isMobile()) {
    g.append("g")
      .attr("class", "axis y")
      .attr("transform", `translate(${width},0)`)
      .call(d3.axisRight(y).tickPadding(5).tickFormat(fmtCrude));
  }

  g.append("g").attr("class", "axis y").call(d3.axisLeft(y).tickSize(-width).tickFormat(fmtCrude));

  const layer = g.selectAll(".layer").data([points]).enter().append("g").attr("class", "layer");

  const clipAboveID = _.uniqueId("clip-above");
  layer
    .append("clipPath")
    .attr("id", clipAboveID)
    .append("path")
    .attr(
      "d",
      area(height, (d) => {
        return y(d.gainAmount + d.investmentAmount - d.withdrawalAmount);
      })
    );

  const clipBelowID = _.uniqueId("clip-below");
  layer
    .append("clipPath")
    .attr("id", clipBelowID)
    .append("path")
    .attr(
      "d",
      area(0, (d) => {
        return y(d.gainAmount + d.investmentAmount - d.withdrawalAmount);
      })
    );

  layer
    .append("path")
    .style("fill", z("gain"))
    .style("opacity", "0.2")
    .attr("clip-path", svgUrl(clipAboveID))
    .attr(
      "d",
      area(0, (d) => {
        return y(d.investmentAmount - d.withdrawalAmount);
      })
    );

  layer
    .append("path")
    .attr("clip-path", svgUrl(clipBelowID))
    .style("fill", z("loss"))
    .style("opacity", "0.2")
    .attr(
      "d",
      area(height, (d) => {
        return y(d.investmentAmount - d.withdrawalAmount);
      })
    );

  layer
    .append("path")
    .style("stroke", lineScale("investment"))
    .style("stroke-width", "1.5")
    .style("fill", "none")
    .attr(
      "d",
      d3
        .line<Networth>()
        .curve(d3.curveMonotoneX)
        .x((d) => x(d.date))
        .y((d) => y(investment(d)))
    );

  layer
    .append("path")
    .style("stroke", lineScale("networth"))
    .style("stroke-width", "1.5")
    .style("fill", "none")
    .attr(
      "d",
      d3
        .line<Networth>()
        .curve(d3.curveMonotoneX)
        .x((d) => x(d.date))
        .y((d) => y(networth(d)))
    );

  const hoverCircle = layer.append("circle").attr("r", "3").attr("fill", "none");
  const t = tippy(hoverCircle.node(), { theme: "light", delay: 0, allowHTML: true });

  const networthVoronoiPoints: Delaunay.Point[] = _.map(points, (d) => [x(d.date), y(networth(d))]);
  const investmentVoronoiPoints: Delaunay.Point[] = _.map(points, (d) => [
    x(d.date),
    y(investment(d))
  ]);
  const voronoi = Delaunay.from(networthVoronoiPoints.concat(investmentVoronoiPoints)).voronoi([
    0,
    0,
    width,
    height
  ]);

  layer
    .append("g")
    .selectAll("path")
    .data(
      points.map((p) => ["networth", p]).concat(points.map((p) => ["investment", p])) as [
        string,
        Networth
      ][]
    )
    .enter()
    .append("path")
    .style("pointer-events", "all")
    .style("fill", "none")
    .attr("d", (_, i) => {
      return voronoi.renderCell(i);
    })
    .on("mouseover", (_, [pointType, d]) => {
      hoverCircle
        .attr("cx", x(d.date))
        .attr("cy", y(pointType == "networth" ? networth(d) : investment(d)))
        .attr("fill", lineScale(pointType));

      t.setProps({
        placement: pointType == "networth" ? "top" : "bottom",
        content: tooltip([
          ["Date", d.date.format("DD MMM YYYY")],
          ["Net Worth", [fmt(networth(d)), "has-text-weight-bold has-text-right"]],
          ["Net Investment", [fmt(investment(d)), "has-text-weight-bold has-text-right"]],
          ["Gain / Loss", [fmt(d.gainAmount), "has-text-weight-bold has-text-right"]]
        ])
      });
      t.show();
    })
    .on("mouseout", () => {
      t.hide();
      hoverCircle.attr("fill", "none");
    });

  const legends: Legend[] = [
    {
      label: "Net Worth",
      color: lineScale("networth"),
      shape: "line"
    },
    {
      label: "Net Investment",
      color: lineScale("investment"),
      shape: "line"
    },
    {
      label: "Gain",
      color: areaScale("gain"),
      shape: "square"
    },
    {
      label: "Loss",
      color: areaScale("loss"),
      shape: "square"
    }
  ];

  const destroy = () => {
    t.destroy();
  };

  return { destroy, legends };
}
