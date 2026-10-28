import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { loadRuntimeConfig, runtimeConfigRequired } from "./runtimeConfig";
import "./styles/global.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("Root element #root not found");
}

async function start() {
  try {
    if (runtimeConfigRequired()) await loadRuntimeConfig();
    const { App } = await import("./App");
    createRoot(root!).render(
      <StrictMode>
        <BrowserRouter><App /></BrowserRouter>
      </StrictMode>,
    );
  } catch {
    const panel = document.createElement("main");
    panel.setAttribute("role", "alert");
    panel.textContent = "Voice Admin configuration is unavailable. ";
    const retry = document.createElement("button");
    retry.type = "button";
    retry.textContent = "Retry loading";
    retry.addEventListener("click", () => { void start(); });
    panel.append(retry);
    root!.replaceChildren(panel);
  }
}

void start();
