import App from "./App.svelte";
import "./styles/reset.scss";
import "./styles/theme.scss";
import "./styles/global.scss";
import "./styles/variables.scss";
import { initializeServerRuntime } from "./thunks/GeneralThunk";

let app;

initializeServerRuntime()
  .catch((error) => console.warn("Failed to load server runtime configuration", error))
  .finally(() => {
    app = new App({
      target: document.body,
    });
  });

export default app;
