import "@fortawesome/fontawesome-free/css/all.min.css";
import "./styles/bootstrap-reboot.min.css";
import "./styles/theme.css";
import "./styles/app.css";

import { mount } from "svelte";
import App from "./App.svelte";

mount(App, { target: document.getElementById("app") });
