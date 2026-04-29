import chroma from "chroma-js";
import _ from "lodash";
import { getColorPreference } from "./utils";
import * as d3 from "d3";

const COLORS = {
  gain: "#9cefc7",
  gainText: "#38c98f",
  loss: "#ffb1ad",
  lossText: "#ff6b6b",
  danger: "#ef4444",
  success: "#109669",
  warn: "#ffd66b",
  warnText: "#c28515",
  diff: "#64748b",

  // accounts
  primary: "#2dd4bf",
  secondary: "#60a5fa",
  tertiary: "#fbbf24",
  neutral: "#94a3b8",
  assets: "#38bdf8",
  expenses: "#fb923c",
  income: "#84cc16",
  liabilities: "#f59e0b",
  equity: "#5b8def"
};
export default COLORS;

type CategoryVisual = {
  icon: string;
  color: string;
  aliases?: string[];
};

const CATEGORY_VISUAL_ENTRIES: Record<string, CategoryVisual> = {
  housing: {
    icon: "fa-house",
    color: "#2dd4bf",
    aliases: ["rent", "home", "mortgage", "house"]
  },
  groceries: {
    icon: "fa-basket-shopping",
    color: "#60a5fa",
    aliases: ["grocery", "supermarket", "food"]
  },
  transport: {
    icon: "fa-bus",
    color: "#f59e0b",
    aliases: ["travel", "commute", "fuel", "mobility"]
  },
  dining: {
    icon: "fa-utensils",
    color: "#fb7185",
    aliases: ["restaurants", "restaurant", "eating out"]
  },
  utilities: {
    icon: "fa-bolt",
    color: "#facc15",
    aliases: ["bills", "electricity", "water", "internet", "phone"]
  },
  shopping: {
    icon: "fa-bag-shopping",
    color: "#a78bfa",
    aliases: ["retail", "amazon", "clothing"]
  },
  health: {
    icon: "fa-heart-pulse",
    color: "#fb7185",
    aliases: ["medical", "doctor", "fitness"]
  },
  entertainment: {
    icon: "fa-film",
    color: "#c084fc",
    aliases: ["fun", "leisure", "streaming"]
  },
  savings: {
    icon: "fa-piggy-bank",
    color: "#34d399",
    aliases: ["saving", "reserve"]
  },
  other: {
    icon: "fa-ellipsis",
    color: "#94a3b8",
    aliases: ["misc", "miscellaneous", "unknown"]
  }
};

const CATEGORY_LOOKUP = _.fromPairs(
  Object.entries(CATEGORY_VISUAL_ENTRIES).flatMap(([key, value]) =>
    [key].concat(value.aliases || []).map((alias) => [alias, key])
  )
);

const CATEGORY_FALLBACKS = [
  "#2dd4bf",
  "#60a5fa",
  "#f59e0b",
  "#fb7185",
  "#a78bfa",
  "#facc15",
  "#34d399",
  "#38bdf8",
  "#f97316",
  "#c084fc"
];

function normalizeCategory(category: string) {
  return category?.toLowerCase().trim() || "other";
}

function resolvedCategoryKey(category: string) {
  return CATEGORY_LOOKUP[normalizeCategory(category)] || normalizeCategory(category);
}

export function categoryVisual(category: string, fallbackIndex = 0): CategoryVisual {
  const key = resolvedCategoryKey(category);
  const known = CATEGORY_VISUAL_ENTRIES[key];
  if (known) {
    return known;
  }

  return {
    icon: "fa-circle",
    color: CATEGORY_FALLBACKS[Math.abs(fallbackIndex) % CATEGORY_FALLBACKS.length]
  };
}

export function categoryColor(category: string, fallbackIndex = 0) {
  return categoryVisual(category, fallbackIndex).color;
}

export function categoryIcon(category: string, fallbackIndex = 0) {
  return categoryVisual(category, fallbackIndex).icon;
}

export function categoryColorScale(categories: string[]) {
  return d3
    .scaleOrdinal<string>()
    .domain(categories)
    .range(categories.map((category, index) => categoryColor(category, index)));
}

export function generateColorScheme(domain: string[]) {
  let colors: string[];
  const n = domain.length;

  if (_.every(domain, (d) => _.has(COLORS, d.toLowerCase()))) {
    colors = _.map(domain, (d) => (COLORS as Record<string, string>)[d.toLowerCase()]);
  } else {
    if (n <= 12) {
      colors = {
        1: ["#7570b3"],
        2: ["#7fc97f", "#fdc086"],
        3: ["#66c2a5", "#fc8d62", "#8da0cb"],
        4: ["#66c2a5", "#fc8d62", "#8da0cb", "#e78ac3"],
        5: ["#66c2a5", "#fc8d62", "#8da0cb", "#e78ac3", "#a6d854"],
        6: ["#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949"],
        7: ["#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1"],
        8: ["#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7"],
        9: [
          "#4e79a7",
          "#f28e2c",
          "#e15759",
          "#76b7b2",
          "#59a14f",
          "#edc949",
          "#af7aa1",
          "#ff9da7",
          "#9c755f"
        ],
        10: [
          "#4e79a7",
          "#f28e2c",
          "#e15759",
          "#76b7b2",
          "#59a14f",
          "#edc949",
          "#af7aa1",
          "#ff9da7",
          "#9c755f",
          "#bab0ab"
        ],
        11: [
          "#8dd3c7",
          "#ffed6f",
          "#bebada",
          "#fb8072",
          "#80b1d3",
          "#fdb462",
          "#b3de69",
          "#fccde5",
          "#d9d9d9",
          "#bc80bd",
          "#ccebc5"
        ],
        12: [
          "#8dd3c7",
          "#e5c494",
          "#bebada",
          "#fb8072",
          "#80b1d3",
          "#fdb462",
          "#b3de69",
          "#fccde5",
          "#d9d9d9",
          "#bc80bd",
          "#ccebc5",
          "#ffed6f"
        ]
      }[n];
    } else {
      const z = d3
        .scaleSequential()
        .domain([0, n - 1])
        .interpolator(d3.interpolateSinebow);
      colors = _.map(_.range(0, n), (n) => chroma(z(n)).desaturate(1.5).hex());
    }
  }

  return d3.scaleOrdinal<string>().domain(domain).range(colors);
}

export function genericBarColor() {
  return getColorPreference() == "dark" ? "hsl(215, 18%, 15%)" : "hsl(0, 0%, 91%)";
}

const OPACITY: Record<string, Record<string, number>> = {
  dark: {
    assets: 0.6,
    expenses: 0.5,
    income: 0.6,
    liabilities: 0.6
  },
  light: {
    assets: 0.8,
    expenses: 0.7,
    income: 1,
    liabilities: 1
  }
};

export function accountColorStyle(account: string) {
  const normalized = account.toLowerCase();
  const opacity = OPACITY[getColorPreference()]?.[normalized] || 1.0;
  let color = "hsl(0, 0%, 48%)";

  if (_.includes(["assets", "expenses", "income", "liabilities", "equity"], normalized)) {
    color = (COLORS as Record<string, string>)[normalized];
  }

  return `color: ${color}; opacity: ${opacity};`;
}

export function white() {
  return getColorPreference() == "dark" ? "hsl(215, 18%, 10%)" : "hsl(0, 0%, 95%)";
}
