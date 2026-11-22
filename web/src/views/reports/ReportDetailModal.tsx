import { Alert, Badge, Group, Loader, Paper, Stack, Text } from "@mantine/core";
import { FileText, Upload } from "lucide-react";
import { useEffect, useState } from "react";
import type { JSX } from "react";

import {
  api,
  type ReportDocumentBody,
  type ReportRefBody,
} from "../../api/client";
import { CappedList } from "../../components/CappedList";
import { IconAction } from "../../components/IconAction";
import { DetailDrawer } from "../../components/DetailDrawer";
import { ImageViewerModal } from "../../components/ImageViewerModal";
import { PdfViewerModal } from "../../components/PdfViewerModal";
import { blobToDataUrl } from "../../lib/blob";
import { formatDate, formatDateTime } from "../../lib/format";
import { useRowQuery } from "../../lib/useRowQuery";
import { LocalReportEditor } from "./LocalReportEditor";
import { VisitLinks } from "./VisitLinks";

interface Props {
  reportRef: ReportRefBody;
  onClose: () => void;
  onChanged?: () => void;
}

interface Viewer {
  kind: "pdf" | "image";
  url: string;
  title: string;
}

interface DocumentsResult {
  documents: readonly ReportDocumentBody[];
  error: string | undefined;
}

const unauthorizedMessage = "Accedi per vedere i referti.";
const documentsMessage = "Documenti non disponibili.";
const rowMessage = "Referto non disponibile.";
const notFoundMessage = "Referto non trovato.";
const pdfMessage = "Impossibile aprire il PDF.";

function failureMessage(status: number, fallback: string): string {
  return status === 401 ? unauthorizedMessage : fallback;
}

export function ReportDetailModal({
  reportRef,
  onClose,
  onChanged,
}: Props): JSX.Element {
  const [result, setResult] = useState<DocumentsResult | undefined>(undefined);
  const [viewer, setViewer] = useState<Viewer | undefined>(undefined);
  const [opening, setOpening] = useState<string | undefined>(undefined);
  const [pdfError, setPdfError] = useState<string | undefined>(undefined);
  const { kind, abstractId, detailId, repCode } = reportRef;
  const rowQuery = useRowQuery("/api/reports/row", { ref: reportRef });
  const row = rowQuery.data?.row;

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.POST("/api/reports/documents", {
          body: { kind, abstractId, detailId, repCode },
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data === undefined) {
          setResult({
            documents: [],
            error: failureMessage(response.status, documentsMessage),
          });
          return;
        }
        setResult({ documents: data.documents ?? [], error: undefined });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setResult({ documents: [], error: documentsMessage });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [kind, abstractId, detailId, repCode]);

  const pdfUrl = viewer?.kind === "pdf" ? viewer.url : undefined;

  useEffect(() => {
    if (pdfUrl === undefined) {
      return undefined;
    }
    return () => {
      URL.revokeObjectURL(pdfUrl);
    };
  }, [pdfUrl]);

  const local = row?.origin === "local";
  const { reload } = rowQuery;
  const notifyChanged = (): void => {
    reload();
    onChanged?.();
  };

  const openPdf = async (document: ReportDocumentBody): Promise<void> => {
    setOpening(document.position);
    setPdfError(undefined);
    try {
      const { data, response } = await api.POST("/api/reports/pdf", {
        body: {
          kind,
          abstractId,
          detailId,
          repCode,
          position: document.position,
        },
        parseAs: "blob",
      });
      if (data === undefined) {
        setPdfError(failureMessage(response.status, pdfMessage));
        return;
      }
      const title =
        document.label === "" ? (row?.kindLabel ?? "Referto") : document.label;
      if (data.type.startsWith("image/")) {
        setViewer({ kind: "image", url: await blobToDataUrl(data), title });
        return;
      }
      setViewer({ kind: "pdf", url: URL.createObjectURL(data), title });
    } catch {
      setPdfError(pdfMessage);
    } finally {
      setOpening(undefined);
    }
  };

  const documents = result?.documents;
  const summary = [row?.provider ?? "", row?.date ? formatDate(row.date) : ""]
    .filter((part) => part !== "")
    .join(" · ");
  const fallbackLabel = local ? "Apri documento" : "Apri PDF";

  const rowError = rowQuery.notFound
    ? notFoundMessage
    : rowQuery.error === "unauthorized"
      ? unauthorizedMessage
      : rowQuery.error === undefined
        ? undefined
        : rowMessage;

  const documentsTab = (
    <>
      {rowError === undefined ? null : (
        <Text c="red" role="alert">
          {rowError}
        </Text>
      )}
      {result === undefined ? (
        <Group gap="xs">
          <Loader size="sm" />
          <Text>Carico i documenti…</Text>
        </Group>
      ) : null}
      {result?.error === undefined ? null : (
        <Text c="red" role="alert">
          {result.error}
        </Text>
      )}
      {result !== undefined &&
      result.error === undefined &&
      result.documents.length === 0 ? (
        <Text>Nessun documento disponibile.</Text>
      ) : null}
      {documents === undefined ? null : (
        <CappedList
          items={documents}
          getKey={(document) => `${document.position}|${document.label}`}
          renderItem={(document) => (
            <Paper withBorder p="xs">
              <Group justify="space-between" wrap="nowrap">
                <Stack gap={0}>
                  <Text>{document.label}</Text>
                  {document.acceptedAt === undefined ? null : (
                    <Text size="xs" c="dimmed">
                      {formatDateTime(document.acceptedAt)}
                    </Text>
                  )}
                </Stack>
                <IconAction
                  label={
                    document.label === ""
                      ? fallbackLabel
                      : `Apri ${document.label}`
                  }
                  icon={FileText}
                  loading={opening === document.position}
                  disabled={opening !== undefined}
                  onClick={() => {
                    void openPdf(document);
                  }}
                />
              </Group>
            </Paper>
          )}
        />
      )}
      {pdfError === undefined ? null : (
        <Alert color="red" role="alert">
          {pdfError}
        </Alert>
      )}
    </>
  );

  return (
    <>
      <DetailDrawer
        title={row?.title ?? "Referto"}
        badge={
          local ? (
            <Badge
              variant="light"
              leftSection={<Upload size={12} aria-hidden="true" />}
            >
              Caricato
            </Badge>
          ) : undefined
        }
        summary={
          summary === "" ? undefined : (
            <Text size="sm" c="dimmed">
              {summary}
            </Text>
          )
        }
        tabs={[
          {
            value: "documenti",
            label: "Documenti",
            ...(documents === undefined ? {} : { count: documents.length }),
            content: documentsTab,
          },
          ...(row === undefined
            ? []
            : [
                {
                  value: "visite",
                  label: "Visite collegate",
                  count: row.visits.length,
                  content: (
                    <VisitLinks
                      reportRef={reportRef}
                      visits={row.visits}
                      onChanged={notifyChanged}
                    />
                  ),
                },
              ]),
          ...(row !== undefined && local
            ? [
                {
                  value: "modifica",
                  label: "Modifica",
                  content: (
                    <LocalReportEditor
                      row={row}
                      onChanged={() => {
                        notifyChanged();
                        onClose();
                      }}
                    />
                  ),
                },
              ]
            : []),
        ]}
        onClose={onClose}
      />
      {viewer?.kind === "pdf" ? (
        <PdfViewerModal
          url={viewer.url}
          title={viewer.title}
          opened
          onClose={() => {
            setViewer(undefined);
          }}
        />
      ) : null}
      {viewer?.kind === "image" ? (
        <ImageViewerModal
          url={viewer.url}
          title={viewer.title}
          opened
          onClose={() => {
            setViewer(undefined);
          }}
        />
      ) : null}
    </>
  );
}
