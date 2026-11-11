import { AppShell, NavLink, createTheme } from "@mantine/core";

import classes from "./theme.module.css";

export const chromeColors = {
  background: "#1b2a41",
  border: "#26385a",
  text: "#ffffff",
  secondaryText: "#b8c4d6",
} as const;

export const tableColors = {
  backgroundColor: { light: "white", dark: "dark.6" },
  borderColor: { light: "gray.3", dark: "dark.4" },
  rowBorderColor: { light: "gray.3", dark: "dark.4" },
  stripedColor: { light: "gray.0", dark: "dark.7" },
  highlightOnHoverColor: { light: "gray.2", dark: "dark.5" },
} as const;

export const theme = createTheme({
  primaryColor: "blue",
  defaultRadius: "md",
  fontFamily: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
  components: {
    AppShell: AppShell.extend({
      styles: {
        main: {
          backgroundColor:
            "light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-8))",
        },
        header: {
          backgroundColor: chromeColors.background,
          borderBottom: `1px solid ${chromeColors.border}`,
          color: chromeColors.text,
        },
        navbar: {
          backgroundColor: chromeColors.background,
          borderRight: `1px solid ${chromeColors.border}`,
          color: chromeColors.text,
        },
      },
    }),
    NavLink: NavLink.extend({
      classNames: { root: classes.navLink },
    }),
  },
});
