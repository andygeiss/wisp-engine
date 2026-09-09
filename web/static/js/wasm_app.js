// Loads the lab and publishes the handful of facts Go cannot reach on its own:
// the module's linear memory, its size in bytes, the GPU's name, and the count
// of frames the browser could not finish in time. The version comes from
// <html data-version>, so every URL is fetched fresh after a rebuild despite
// the one-year immutable cache.

// wisp is the whole bridge. Go reads it and never writes to it. It is declared
// before anything else runs, so a read can never find it half built.
globalThis.wisp = {
  gpu: gpuInfo(), // read once: creating a WebGL context is not free
  longFrameMs: 0,
  longFrames: 0,
  longFramesKind: "none",
  memory: null, // WebAssembly.Memory; .buffer.byteLength is the linear memory
  wasmBytes: 0, // the module's own size: the number this project protects
};

observeLongFrames();
boot();

async function boot() {
  // Declared in here, not at the top level: two classic scripts share one
  // scope, and a top-level const that collides with wasm_exec.js is a syntax
  // error that takes the whole page down.
  const version = document.documentElement.dataset.version || "";
  const url = "/static/lab.wasm" + (version ? "?v=" + version : "");
  const go = new Go(); // defined in wasm_exec.js

  try {
    // Not instantiateStreaming: reading the bytes first is what lets the
    // overlay show the module's own size, which is the number this project
    // exists to keep small. On localhost the lost overlap is microseconds.
    const res = await fetch(url);
    if (!res.ok) throw new Error(res.status + " " + res.statusText);
    const bytes = await res.arrayBuffer();
    globalThis.wisp.wasmBytes = bytes.byteLength;

    const result = await WebAssembly.instantiate(bytes, go.importObject);
    globalThis.wisp.memory = result.instance.exports.memory;
    go.run(result.instance);
  } catch (err) {
    // Nothing Go can draw on exists yet, so the loader draws its own message —
    // on a canvas, because overlay UI is never DOM here.
    fail(url + "\n" + err.message + "\n\nRun make wasm, then reload.");
  }
}

// gpuInfo names the GPU when the browser allows it. Chromium returns real
// strings through WEBGL_debug_renderer_info; Firefox hides them under
// resistFingerprinting and Safari reports generic values, so "masked" is what
// the overlay prints when the name cannot be trusted.
function gpuInfo() {
  const c = document.createElement("canvas");
  const gl = c.getContext("webgl2") || c.getContext("webgl");
  if (!gl) return { masked: true, renderer: "n/a", vendor: "n/a" };

  const ext = gl.getExtension("WEBGL_debug_renderer_info");
  const info = ext
    ? {
        masked: false,
        renderer: gl.getParameter(ext.UNMASKED_RENDERER_WEBGL),
        vendor: gl.getParameter(ext.UNMASKED_VENDOR_WEBGL),
      }
    : {
        masked: true,
        renderer: gl.getParameter(gl.RENDERER),
        vendor: gl.getParameter(gl.VENDOR),
      };

  // The lab draws with Canvas2D. Holding a WebGL context open would keep a
  // second GPU context — and on a laptop sometimes a second GPU — awake for a
  // string we already have.
  const lose = gl.getExtension("WEBGL_lose_context");
  if (lose) lose.loseContext();
  return info;
}

// observeLongFrames counts frames the browser could not finish in time.
// long-animation-frame includes style, layout and paint, so it sees a slow
// canvas; longtask only sees script. Neither exists outside Chromium, which is
// why the overlay says n/a there rather than 0.
function observeLongFrames() {
  if (typeof PerformanceObserver === "undefined") return;
  const kinds = PerformanceObserver.supportedEntryTypes || [];
  const kind = kinds.includes("long-animation-frame")
    ? "long-animation-frame"
    : kinds.includes("longtask")
      ? "longtask"
      : null;
  if (!kind) return;

  globalThis.wisp.longFramesKind = kind;
  new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      globalThis.wisp.longFrames++;
      globalThis.wisp.longFrameMs += entry.duration;
    }
  }).observe({ buffered: true, type: kind });
}

// fail draws the reason on a canvas, so the one rule about overlay UI holds
// even when Go never started.
function fail(message) {
  const canvas = document.createElement("canvas");
  canvas.width = 640;
  canvas.height = 360;
  (document.querySelector("main") || document.body).appendChild(canvas);

  const ctx = canvas.getContext("2d");
  ctx.fillStyle = "#12141a";
  ctx.fillRect(0, 0, 640, 360);
  ctx.fillStyle = "#ff6b6b";
  ctx.font = "16px system-ui, sans-serif";
  ctx.textBaseline = "top";
  ctx.fillText("The lab did not load.", 24, 24);
  ctx.font = "12px ui-monospace, monospace";
  ctx.fillStyle = "#e6e6e6";
  message.split("\n").forEach((line, i) => ctx.fillText(line, 24, 60 + i * 16));
}
