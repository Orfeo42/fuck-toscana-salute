import { Alert, Anchor, Loader, Modal, Stack } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { useEffect, useRef, useState } from "react";
import type { JSX } from "react";

import { loadPdf } from "../lib/pdf";
import type { PdfDocument } from "../lib/pdf";

const FALLBACK_WIDTH = 600;

interface Props {
  url: string;
  title: string;
  opened: boolean;
  onClose: () => void;
}

interface PageProps {
  doc: PdfDocument;
  pageNumber: number;
}

interface BodyProps {
  url: string;
}

function NewTabLink({ url }: BodyProps): JSX.Element {
  return (
    <Anchor href={url} target="_blank" rel="noopener noreferrer">
      Apri in una nuova scheda
    </Anchor>
  );
}

function PdfPageCanvas({ doc, pageNumber }: PageProps): JSX.Element {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (canvas === null) {
      return;
    }
    let cancelled = false;
    let cancelRender: (() => void) | undefined;

    doc
      .getPage(pageNumber)
      .then((page) => {
        if (cancelled) {
          return undefined;
        }
        const containerWidth =
          canvas.parentElement?.clientWidth ?? FALLBACK_WIDTH;
        const width = containerWidth > 0 ? containerWidth : FALLBACK_WIDTH;
        const cssScale = width / page.getViewport({ scale: 1 }).width;
        const viewport = page.getViewport({
          scale: cssScale * window.devicePixelRatio,
        });
        canvas.width = viewport.width;
        canvas.height = viewport.height;
        const task = page.render({ canvas, viewport });
        cancelRender = task.cancel;
        return task.promise;
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
        }
      });

    return () => {
      cancelled = true;
      cancelRender?.();
    };
  }, [doc, pageNumber]);

  if (failed) {
    return <Alert color="red">Pagina {pageNumber} non disponibile</Alert>;
  }
  return (
    <canvas
      ref={canvasRef}
      aria-label={`Pagina ${String(pageNumber)} di ${String(doc.numPages)}`}
      role="img"
      style={{ width: "100%", height: "auto", display: "block" }}
    />
  );
}

function PdfBody({ url }: BodyProps): JSX.Element {
  const [doc, setDoc] = useState<PdfDocument | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const task = loadPdf(url);

    task.promise
      .then((loaded) => {
        if (!cancelled) {
          setDoc(loaded);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
        }
      });

    return () => {
      cancelled = true;
      task.destroy().catch(() => undefined);
    };
  }, [url]);

  if (failed) {
    return (
      <Stack gap="sm">
        <Alert color="red" title="Impossibile aprire il PDF">
          Il documento non è disponibile.
        </Alert>
        <NewTabLink url={url} />
      </Stack>
    );
  }
  if (doc === null) {
    return (
      <Stack gap="sm">
        <NewTabLink url={url} />
        <Stack align="center" py="xl">
          <Loader aria-label="Caricamento PDF" />
        </Stack>
      </Stack>
    );
  }
  return (
    <Stack gap="sm">
      <NewTabLink url={url} />
      {Array.from({ length: doc.numPages }, (_, index) => (
        <PdfPageCanvas key={index + 1} doc={doc} pageNumber={index + 1} />
      ))}
    </Stack>
  );
}

export function PdfViewerModal({
  url,
  title,
  opened,
  onClose,
}: Props): JSX.Element {
  const fullScreen = useMediaQuery("(max-width: 48em)");
  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={title}
      size="xl"
      fullScreen={fullScreen}
      closeButtonProps={{ "aria-label": "Chiudi" }}
    >
      <PdfBody url={url} />
    </Modal>
  );
}
