import _ from "lodash";
import { createReadStream, mkdirSync, readdirSync, writeFileSync } from "fs";
import { fileURLToPath } from "url";
import { dirname, join } from "path";
import svg2ttf from "svg2ttf";
import ttf2woff2 from "ttf2woff2";
import { SVGIcons2SVGFontStream } from "svgicons2svgfont";

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
process.chdir(repoRoot);

const outputDir = "fonts";

let infoData = {};
let lastUnicode = 0xf0000;

function svgFiles(dir) {
  return readdirSync(dir)
    .filter((entry) => entry.endsWith(".svg"))
    .sort((left, right) => left.localeCompare(right));
}

function createSvgFont(font, dir, startUnicode) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    const fontStream = new SVGIcons2SVGFontStream({
      fontName: font,
      fontId: font,
      normalize: true,
      fontWeight: 900,
      fontHeight: 1000,
      fontStyle: "normal",
      fixedWidth: true,
      centerHorizontally: true,
      centerVertically: true
    });

    let nextCodePoint = startUnicode;

    fontStream.on("data", (chunk) => {
      chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
    });
    fontStream.on("error", reject);
    fontStream.on("finish", () => {
      lastUnicode = nextCodePoint;
      resolve(Buffer.concat(chunks).toString("utf8"));
    });

    for (const file of svgFiles(dir)) {
      const name = file.slice(0, -4);
      nextCodePoint += 1;
      infoData[name] = nextCodePoint;

      const glyph = createReadStream(join(dir, file));
      glyph.metadata = {
        name,
        unicode: [String.fromCodePoint(nextCodePoint)]
      };
      glyph.on("error", reject);
      fontStream.write(glyph);
    }

    fontStream.end();
  });
}

async function createFont(font) {
  infoData = {};

  const sourceDir = join("svg", font);
  const svgFont = await createSvgFont(font, sourceDir, lastUnicode);
  const ttf = svg2ttf(svgFont, {
    id: font,
    familyname: font,
    fullname: font,
    version: "1.0"
  });
  const ttfBuffer = Buffer.from(ttf.buffer);
  const woff2 = ttf2woff2(ttfBuffer);

  writeFileSync(join(outputDir, `${font}.svg`), svgFont, "utf8");
  writeFileSync(join(outputDir, `${font}.woff2`), woff2);
  writeFileSync(
    join(outputDir, `${font}-info.json`),
    JSON.stringify({ codepoints: infoData }),
    "utf8"
  );

  const min = _.min(Object.values(infoData));
  const max = _.max(Object.values(infoData));
  const scss = `
@font-face {
  font-family: "${font}";
  font-style: normal;
  font-weight: 900;
  src: url("../fonts/${font}.woff2") format("woff2");
  unicode-range: U+${min.toString(16).toUpperCase()}-${max.toString(16).toUpperCase()};
  font-display: block;
}
`;

  writeFileSync(join(outputDir, `${font}.scss`), scss, "utf8");
}

async function createFonts(fonts) {
  mkdirSync(outputDir, { recursive: true });

  for (const font of fonts) {
    await createFont(font);
  }
}

const fonts = [
  "arcticons",
  "fa6-solid",
  "fa6-regular",
  "fa6-brands",
  "mdi",
  "fluent-emoji-high-contrast"
];

createFonts(fonts);
