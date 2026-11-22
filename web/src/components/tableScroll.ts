export const tableMaxHeight = "75vh";

export const tableScrollProps = {
  maxHeight: tableMaxHeight,
  scrollAreaProps: {
    viewportProps: { style: { maxHeight: `calc(${tableMaxHeight} - 2px)` } },
  },
} as const;
