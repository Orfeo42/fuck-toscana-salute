import {
  AppShell,
  NavLink,
  createTheme,
  type MantineColorsTuple,
} from "@mantine/core";

import classes from "./theme.module.css";

const navy: MantineColorsTuple = [
  "#e8eef8",
  "#d0dcee",
  "#a6bbdb",
  "#7b98c4",
  "#5578ad",
  "#3a5f96",
  "#254a80",
  "#1a3a6b",
  "#13315c",
  "#0b2545",
];

const dark: MantineColorsTuple = [
  "#d7e0ee",
  "#c0cde0",
  "#a3b4cf",
  "#7d92b3",
  "#2c4a7a",
  "#24416d",
  "#1b3560",
  "#16305a",
  "#13315c",
  "#0b2545",
];

export const tableColors = {
  backgroundColor: { light: "white", dark: "navy.8" },
  borderColor: { light: "navy.1", dark: "navy.7" },
  rowBorderColor: { light: "navy.1", dark: "navy.7" },
  stripedColor: { light: "navy.0", dark: "navy.9" },
  highlightOnHoverColor: { light: "navy.1", dark: "navy.7" },
} as const;

export const theme = createTheme({
  primaryColor: "blue",
  defaultRadius: "md",
  fontFamily: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
  colors: { navy, dark },
  components: {
    AppShell: AppShell.extend({
      styles: {
        main: {
          backgroundColor: "light-dark(#f3f6fb, var(--mantine-color-navy-9))",
        },
        header: {
          backgroundColor: "var(--mantine-color-navy-8)",
          borderBottom: "1px solid var(--mantine-color-navy-7)",
          color: "var(--mantine-color-white)",
        },
        navbar: {
          backgroundColor: "var(--mantine-color-navy-8)",
          borderRight: "1px solid var(--mantine-color-navy-7)",
          color: "var(--mantine-color-white)",
        },
      },
    }),
    NavLink: NavLink.extend({
      classNames: { root: classes.navLink },
    }),
  },
});
