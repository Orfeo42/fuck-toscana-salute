import {
  ActionIcon,
  Button,
  Group,
  Select,
  Stack,
  Text,
  Tooltip,
  UnstyledButton,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { Unlink, Upload } from "lucide-react";
import { useState, type JSX } from "react";

import {
  api,
  type AppointmentBody,
  type ReportBody,
  type RowBody,
} from "../../api/client";
import { CappedList } from "../../components/CappedList";
import { formatDate } from "../../lib/format";
import { ReportDetailModal } from "../reports/ReportDetailModal";
import { UploadReportDialog } from "../reports/UploadReportDialog";
import { rowTitle, unitName, type EffectiveState } from "./filter";
import { romeDay } from "./relativeDay";
import { linkedReports, reportKey, reportLabel } from "./reportLinks";

interface Props {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  now: Date;
  reports: readonly ReportBody[];
  onChanged: () => void;
}

function optionLabel(report: ReportBody): string {
  return `${reportLabel(report)} · ${formatDate(report.date)} · ${report.provider}`;
}

export function VisitReportsSection({
  row,
  state,
  booking,
  now,
  reports,
  onChanged,
}: Props): JSX.Element {
  const [openReport, setOpenReport] = useState<ReportBody | undefined>(
    undefined,
  );
  const [uploading, setUploading] = useState(false);
  const linked = linkedReports(row, reports);
  const linkedKeys = new Set(linked.map(reportKey));
  const options = reports
    .filter((report) => !linkedKeys.has(reportKey(report)))
    .map((report) => ({
      value: reportKey(report),
      label: optionLabel(report),
    }));
  const appointmentAt = state.kind === "booked" ? state.at : undefined;
  const unit = state.kind === "booked" ? booking?.unit : undefined;
  const defaultDate = romeDay(
    appointmentAt === undefined ? now : new Date(appointmentAt),
  );

  const link = async (key: string): Promise<void> => {
    const report = reports.find((item) => reportKey(item) === key);
    if (report === undefined) {
      return;
    }
    const { kind, abstractId, detailId, repCode } = report;
    try {
      const { response } = await api.PUT("/api/links", {
        body: { nre: row.nre, report: { kind, abstractId, detailId, repCode } },
      });
      if (!response.ok) {
        notifications.show({
          color: "red",
          message: "Impossibile collegare il referto",
        });
        return;
      }
      onChanged();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile collegare il referto",
      });
    }
  };

  const unlink = async (report: ReportBody): Promise<void> => {
    const { kind, abstractId, detailId, repCode } = report;
    try {
      const { response } = await api.DELETE("/api/links", {
        body: { nre: row.nre, report: { kind, abstractId, detailId, repCode } },
      });
      if (!response.ok) {
        notifications.show({
          color: "red",
          message: "Impossibile scollegare il referto",
        });
        return;
      }
      onChanged();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile scollegare il referto",
      });
    }
  };

  return (
    <Stack gap="xs">
      {linked.length === 0 ? (
        <Text size="sm" c="dimmed">
          Nessun referto collegato.
        </Text>
      ) : null}
      <CappedList
        items={linked}
        getKey={reportKey}
        renderItem={(report) => (
          <Group gap="xs" wrap="nowrap">
            {report.origin === "local" ? (
              <Tooltip label="Caricato da te" withArrow openDelay={150}>
                <Upload size={16} role="img" aria-label="Caricato da te" />
              </Tooltip>
            ) : null}
            <UnstyledButton
              aria-label={`Apri referto ${reportLabel(report)}`}
              onClick={() => {
                setOpenReport(report);
              }}
            >
              <Text size="sm" span>
                {`${reportLabel(report)} · ${formatDate(report.date)} · ${report.provider}`}
              </Text>
            </UnstyledButton>
            <Tooltip label="Scollega referto" withArrow openDelay={150}>
              <ActionIcon
                variant="subtle"
                aria-label={`Scollega referto ${reportLabel(report)}`}
                onClick={() => {
                  void unlink(report);
                }}
              >
                <Unlink size={16} aria-hidden="true" />
              </ActionIcon>
            </Tooltip>
          </Group>
        )}
      />
      <Select
        label="Collega referto"
        placeholder="Cerca un referto"
        data={options}
        value={null}
        searchable
        nothingFoundMessage="Nessun referto disponibile"
        onChange={(next) => {
          if (next !== null) {
            void link(next);
          }
        }}
      />
      <Button
        variant="light"
        w="fit-content"
        leftSection={<Upload size={16} aria-hidden="true" />}
        onClick={() => {
          setUploading(true);
        }}
      >
        Carica referto
      </Button>
      {uploading ? (
        <UploadReportDialog
          opened
          onClose={() => {
            setUploading(false);
          }}
          onUploaded={() => {
            setUploading(false);
            onChanged();
          }}
          nre={row.nre}
          defaultTitle={rowTitle(row)}
          defaultDate={defaultDate}
          {...(unit === undefined ? {} : { defaultProvider: unitName(unit) })}
        />
      ) : null}
      {openReport === undefined ? null : (
        <ReportDetailModal
          report={openReport}
          onClose={() => {
            setOpenReport(undefined);
          }}
          onChanged={onChanged}
        />
      )}
    </Stack>
  );
}
