import { GlobalWorkerOptions, getDocument } from "pdfjs-dist";
import type { PageViewport } from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";

GlobalWorkerOptions.workerSrc = workerUrl;

export type PdfViewport = PageViewport;

export interface PdfRenderTask {
  promise: Promise<void>;
  cancel: () => void;
}

export interface PdfPage {
  getViewport: (options: { scale: number }) => PdfViewport;
  render: (options: {
    canvas: HTMLCanvasElement;
    viewport: PdfViewport;
  }) => PdfRenderTask;
}

export interface PdfDocument {
  numPages: number;
  getPage: (pageNumber: number) => Promise<PdfPage>;
}

export interface PdfLoadingTask {
  promise: Promise<PdfDocument>;
  destroy: () => Promise<void>;
}

export function loadPdf(url: string): PdfLoadingTask {
  return getDocument({ url });
}
