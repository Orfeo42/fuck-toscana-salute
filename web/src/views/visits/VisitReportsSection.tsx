import {
  Group,
  Select,
  Stack,
  Text,
  Tooltip,
  UnstyledButton,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { FileText, FileUp, Unlink, Upload } from "lucide-react";
import { useEffect, useState, type JSX } from "react";

import {
  api,
  type LinkedReportBody,
  type ReportOptionBody,
  type ReportRefBody,
  type VisitRowBody,
} from "../../api/client";
import { CappedList } from "../../components/CappedList";
import { IconAction } from "../../components/IconAction";
import { formatDate } from "../../lib/format";
import { ReportDetailModal } from "../reports/ReportDetailModal";
import { UploadReportDialog } from "../reports/UploadReportDialog";

interface Props {
  row: VisitRowBody;
  onChanged: () => void;
}

function refKey(ref: ReportRefBody): string {
  return `${ref.kind}|${ref.abstractId}|${ref.detailId}|${ref.repCode}`;
}

function reportLine(report: {
  label: string;
  date?: string | undefined;
  provider: string;
}): string {
  return `${report.label} · ${formatDate(report.date)} · ${report.provider}`;
}

export function VisitReportsSection({ row, onChanged }: Props): JSX.Element {
  const { nre } = row;
  const [openReport, setOpenReport] = useState<ReportRefBody | undefined>(
    undefined,
  );
  const [uploading, setUploading] = useState(false);
  const [options, setOptions] = useState<readonly ReportOptionBody[]>([]);
  const [optionsVersion, setOptionsVersion] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const load = async (): Promise<void> => {
      try {
        const { data } = await api.POST("/api/visits/report-options", {
          body: { nre },
          signal: controller.signal,
        });
        if (controller.signal.aborted || data === undefined) {
          return;
        }
        setOptions(data.options);
      } catch {
        return;
      }
    };
    void load();
    return () => {
      controller.abort();
    };
  }, [nre, optionsVersion]);

  const changed = (): void => {
    setOptionsVersion((version) => version + 1);
    onChanged();
  };

  const link = async (index: number): Promise<void> => {
    const option = options[index];
    if (option === undefined) {
      return;
    }
    try {
      const { response } = await api.PUT("/api/links", {
        body: { nre, report: option.ref },
      });
      if (!response.ok) {
        notifications.show({
          color: "red",
          message: "Impossibile collegare il referto",
        });
        return;
      }
      changed();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile collegare il referto",
      });
    }
  };

  const unlink = async (report: LinkedReportBody): Promise<void> => {
    try {
      const { response } = await api.DELETE("/api/links", {
        body: { nre, report: report.ref },
      });
      if (!response.ok) {
        notifications.show({
          color: "red",
          message: "Impossibile scollegare il referto",
        });
        return;
      }
      changed();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile scollegare il referto",
      });
    }
  };

  return (
    <Stack gap="xs">
      {row.reports.length === 0 ? (
        <Text size="sm" c="dimmed">
          Nessun referto collegato.
        </Text>
      ) : null}
      <CappedList
        items={row.reports}
        getKey={(report) => refKey(report.ref)}
        renderItem={(report) => (
          <Group gap="xs" wrap="nowrap">
            {report.origin === "local" ? (
              <Tooltip label="Caricato da te" withArrow openDelay={150}>
                <Upload size={16} role="img" aria-label="Caricato da te" />
              </Tooltip>
            ) : null}
            <UnstyledButton
              disabled={!report.available}
              onClick={() => {
                setOpenReport(report.ref);
              }}
            >
              <Text size="sm" span>
                {reportLine(report)}
              </Text>
            </UnstyledButton>
            <IconAction
              label={`Apri referto ${report.label}`}
              icon={FileText}
              disabled={!report.available}
              onClick={() => {
                setOpenReport(report.ref);
              }}
            />
            <IconAction
              label={`Scollega referto ${report.label}`}
              icon={Unlink}
              onClick={() => {
                void unlink(report);
              }}
            />
          </Group>
        )}
      />
      <Select
        label="Collega referto"
        placeholder="Cerca un referto"
        data={options.map((option, index) => ({
          value: String(index),
          label: reportLine(option),
        }))}
        value={null}
        searchable
        nothingFoundMessage="Nessun referto disponibile"
        onChange={(next) => {
          if (next !== null) {
            void link(Number(next));
          }
        }}
      />
      <Group>
        <IconAction
          label="Carica referto"
          icon={FileUp}
          onClick={() => {
            setUploading(true);
          }}
        />
      </Group>
      {uploading ? (
        <UploadReportDialog
          opened
          onClose={() => {
            setUploading(false);
          }}
          onUploaded={() => {
            setUploading(false);
            changed();
          }}
          nre={nre}
          defaultTitle={row.upload.title}
          defaultDate={row.upload.date}
          defaultProvider={row.upload.provider}
        />
      ) : null}
      {openReport === undefined ? null : (
        <ReportDetailModal
          reportRef={openReport}
          onClose={() => {
            setOpenReport(undefined);
          }}
          onChanged={changed}
        />
      )}
    </Stack>
  );
}
