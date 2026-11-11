import { Button, Group, Loader, Modal, Stack, Text } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { FileText } from "lucide-react";
import { useEffect, useState } from "react";
import type { JSX } from "react";

import {
  api,
  type ReportBody,
  type ReportDocumentBody,
} from "../../api/client";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { PdfViewerModal } from "../../components/PdfViewerModal";
import { formatDate, formatDateTime } from "../../lib/format";

interface Props {
  report: ReportBody;
  onClose: () => void;
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

export function ReportDetailModal({ report, onClose }: Props): JSX.Element {
  const fullScreen = useMediaQuery("(max-width: 48em)");
  const [result, setResult] = useState<DocumentsResult | undefined>(undefined);
  const [pdfUrl, setPdfUrl] = useState<string | undefined>(undefined);
  const [pdfTitle, setPdfTitle] = useState("");
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

  useEffect(() => {
    if (pdfUrl === undefined) {
      return undefined;
    }
    return () => {
      URL.revokeObjectURL(pdfUrl);
    };
  }, [pdfUrl]);

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
      setPdfTitle(document.label === "" ? report.kindLabel : document.label);
      setPdfUrl(URL.createObjectURL(data));
    } catch {
      setPdfError(pdfMessage);
    } finally {
      setOpening(undefined);
    }
  };

  return (
    <Modal
      opened
      onClose={onClose}
      title={`${report.kindLabel} · ${formatDate(report.date)}`}
      fullScreen={fullScreen}
      centered
    >
      <Stack gap="lg">
        <DetailSection title="Struttura">
          <DetailField label="Struttura">{report.provider}</DetailField>
        </DetailSection>
        <DetailSection title="Documenti">
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
          {result?.documents.map((document) => (
            <Group
              key={`${document.position}|${document.label}`}
              justify="space-between"
              wrap="nowrap"
            >
              <Stack gap={0}>
                <Text>{document.label}</Text>
                {document.acceptedAt === undefined ? null : (
                  <Text size="xs" c="dimmed">
                    {formatDateTime(document.acceptedAt)}
                  </Text>
                )}
              </Stack>
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
                Apri PDF
              </Button>
            </Group>
          ))}
          {pdfError === undefined ? null : (
            <Text c="red" role="alert">
              {pdfError}
            </Text>
          )}
        </DetailSection>
      </Stack>
      {pdfUrl === undefined ? null : (
        <PdfViewerModal
          url={pdfUrl}
          title={pdfTitle}
          opened
          onClose={() => {
            setPdfUrl(undefined);
          }}
        />
      )}
    </Modal>
  );
}
