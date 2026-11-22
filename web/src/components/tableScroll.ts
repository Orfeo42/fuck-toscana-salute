export const tableMaxHeight = "75vh";

export const tableScrollProps = {
  maxHeight: tableMaxHeight,
  scrollAreaProps: {
    style: { display: "flex", flexDirection: "column", minHeight: 0 },
    viewportProps: { style: { flex: "1 1 auto", minHeight: 0 } },
  },
} as const;
