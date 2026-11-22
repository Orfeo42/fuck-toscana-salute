import {
  Alert,
  Badge,
  Button,
  Group,
  Loader,
  Paper,
  Stack,
  Text,
} from "@mantine/core";
import { FileText, Upload } from "lucide-react";
import { useEffect, useState } from "react";
import type { JSX } from "react";

import {
  api,
  type ReportBody,
  type ReportDocumentBody,
} from "../../api/client";
import { CappedList } from "../../components/CappedList";
import { DetailDrawer } from "../../components/DetailDrawer";
import { ImageViewerModal } from "../../components/ImageViewerModal";
import { PdfViewerModal } from "../../components/PdfViewerModal";
import { blobToDataUrl } from "../../lib/blob";
import { formatDate, formatDateTime } from "../../lib/format";
import { LocalReportEditor } from "./LocalReportEditor";
import { VisitLinks } from "./VisitLinks";

interface Props {
  report: ReportBody;
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
const pdfMessage = "Impossibile aprire il PDF.";

function failureMessage(status: number, fallback: string): string {
  return status === 401 ? unauthorizedMessage : fallback;
}

export function ReportDetailModal({
  report,
  onClose,
  onChanged,
}: Props): JSX.Element {
  const [result, setResult] = useState<DocumentsResult | undefined>(undefined);
  const [viewer, setViewer] = useState<Viewer | undefined>(undefined);
  const [nres, setNres] = useState<readonly string[]>(report.nres ?? []);
  const [opening, setOpening] = useState<string | undefined>(undefined);
  const [pdfError, setPdfError] = useState<string | undefined>(undefined);
  const { kind, abstractId, detailId, repCode } = report;

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

  const local = report.origin === "local";
  const reportRef = { kind, abstractId, detailId, repCode };
  const notifyChanged = (): void => {
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
      const title = document.label === "" ? report.kindLabel : document.label;
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
  const singleDocument = documents?.length === 1 ? documents[0] : undefined;
  const summary = [report.provider, report.date ? formatDate(report.date) : ""]
    .filter((part) => part !== "")
    .join(" · ");
  const openLabel = local ? "Apri documento" : "Apri PDF";

  const documentsTab = (
    <>
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
                {documents.length === 1 ? null : (
                  <Button
                    variant="light"
                    size="compact-sm"
                    leftSection={<FileText size={16} aria-hidden="true" />}
                    loading={opening === document.position}
                    disabled={opening !== undefined}
                    onClick={() => {
                      void openPdf(document);
                    }}
                  >
                    {local ? "Apri" : "Apri PDF"}
                  </Button>
                )}
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
        title={local ? (report.title ?? report.kindLabel) : report.kindLabel}
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
        actions={
          singleDocument === undefined ? undefined : (
            <Button
              size="compact-md"
              leftSection={<FileText size={16} aria-hidden="true" />}
              loading={opening === singleDocument.position}
              disabled={opening !== undefined}
              onClick={() => {
                void openPdf(singleDocument);
              }}
            >
              {openLabel}
            </Button>
          )
        }
        tabs={[
          {
            value: "documenti",
            label: "Documenti",
            ...(documents === undefined ? {} : { count: documents.length }),
            content: documentsTab,
          },
          {
            value: "visite",
            label: "Visite collegate",
            count: nres.length,
            content: (
              <VisitLinks
                reportRef={reportRef}
                nres={nres}
                onLinked={(nre) => {
                  setNres([...nres, nre]);
                  notifyChanged();
                }}
                onUnlinked={(nre) => {
                  setNres(nres.filter((item) => item !== nre));
                  notifyChanged();
                }}
              />
            ),
          },
          ...(local
            ? [
                {
                  value: "modifica",
                  label: "Modifica",
                  content: (
                    <LocalReportEditor
                      report={report}
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
