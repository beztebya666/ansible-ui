import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import "./index.css";
import { isDemo, installDemo } from "./lib/demo";

// Demo build: swap in the in-browser mock backend before anything fetches.
if (isDemo()) installDemo();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
