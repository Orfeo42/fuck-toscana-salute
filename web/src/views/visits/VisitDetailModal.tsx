import { Loader, Paper, Stack, Text } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { MapPinned, Navigation, RefreshCw } from "lucide-react";
import type { JSX } from "react";

import { CopyButton } from "../../components/CopyButton";
import { DetailDrawer, type DetailTab } from "../../components/DetailDrawer";
import { DetailField, DetailSection } from "../../components/DetailSection";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { startRefresh } from "../../data/refresh";
import {
  formatDate,
  formatLongDateTime,
  formatShortDateTime,
} from "../../lib/format";
import { useRowQuery } from "../../lib/useRowQuery";
import { BookAction } from "./BookAction";
import { CalendarAction } from "./CalendarAction";
import { PromemoriaLink } from "./PromemoriaLink";
import { VisitReportsSection } from "./VisitReportsSection";
import { VisitStatusSection } from "./VisitStatusSection";

interface Props {
  nre: string;
  onClose: () => void;
  onChanged: () => void;
}

export function VisitDetailModal({
  nre,
  onClose,
  onChanged,
}: Props): JSX.Element {
  const query = useRowQuery("/api/visits/row", { nre });
  const { data, reload } = query;

  if (data === undefined) {
    const message = query.notFound
      ? "Visita non trovata."
      : query.error === undefined
        ? undefined
        : "Impossibile caricare la visita.";
    return (
      <DetailDrawer
        title="Visita"
        tabs={[
          {
            value: "panoramica",
            label: "Panoramica",
            content:
              message === undefined ? (
                <Loader size="sm" aria-label="Carico la visita" />
              ) : (
                <Text size="sm">{message}</Text>
              ),
          },
        ]}
        onClose={onClose}
      />
    );
  }

  const { row, manualStatus } = data;
  const { appointment, status } = row;
  const unit = appointment?.unit;
  const at = appointment?.at;
  const updates = [
    row.checkedAt.appointments === null
      ? undefined
      : `Appuntamenti: ${formatShortDateTime(row.checkedAt.appointments)}`,
    row.checkedAt.verdict === null
      ? undefined
      : `Verifica CUP: ${formatShortDateTime(row.checkedAt.verdict)}`,
  ].filter((item) => item !== undefined);

  const changed = (): void => {
    reload();
    onChanged();
  };

  const refresh = async (): Promise<void> => {
    try {
      const started = await startRefresh({ nres: [nre] });
      if (!started) {
        notifications.show({
          color: "red",
          message: "Impossibile avviare l'aggiornamento",
        });
        return;
      }
      changed();
    } catch {
      notifications.show({
        color: "red",
        message: "Impossibile avviare l'aggiornamento",
      });
    }
  };

  const tabs: DetailTab[] = [
    {
      value: "panoramica",
      label: "Panoramica",
      content: (
        <>
          <Paper withBorder p="sm" radius="md">
            <DetailSection
              title="Stato"
              actions={status.canBook ? <BookAction /> : undefined}
            >
              <Stack gap={4}>
                {status.details.map((detail) => (
                  <Text key={detail} size="sm">
                    {detail}
                  </Text>
                ))}
              </Stack>
              <VisitStatusSection
                nre={nre}
                manualStatus={manualStatus}
                onChanged={changed}
              />
            </DetailSection>
          </Paper>
          {appointment === undefined || unit === undefined ? null : (
            <Paper withBorder p="sm" radius="md">
              <DetailSection
                title="Appuntamento"
                actions={
                  <>
                    <PromemoriaLink bookingId={appointment.bookingId} />
                    {appointment.upcoming ? (
                      <CalendarAction appointment={appointment} />
                    ) : null}
                    {unit.address === "" ? null : (
                      <>
                        <IconAction
                          label="Indicazioni stradali"
                          icon={Navigation}
                          href={unit.mapsDirectionsUrl}
                          external
                        />
                        <IconAction
                          label="Apri in Google Maps"
                          icon={MapPinned}
                          href={unit.mapsSearchUrl}
                          external
                        />
                      </>
                    )}
                  </>
                }
              >
                {at === undefined ? null : (
                  <DetailField label="Data e ora">
                    <>
                      <Text>{formatLongDateTime(at)}</Text>
                      {appointment.relativeDay === undefined ? null : (
                        <Text size="sm" c="dimmed">
                          {appointment.relativeDay}
                        </Text>
                      )}
                    </>
                  </DetailField>
                )}
                <DetailField label="Struttura">
                  <>
                    <Text>{unit.displayName}</Text>
                    {unit.description !== "" &&
                    unit.description !== unit.name ? (
                      <Text size="sm" c="dimmed">
                        {unit.name}
                      </Text>
                    ) : null}
                    {unit.place === undefined || unit.place === "" ? null : (
                      <Text size="sm">{unit.place}</Text>
                    )}
                    {unit.fullAddress === "" ? null : (
                      <Text size="sm">{unit.fullAddress}</Text>
                    )}
                  </>
                </DetailField>
              </DetailSection>
            </Paper>
          )}
        </>
      ),
    },
    {
      value: "referti",
      label: "Referti",
      count: row.reports.length,
      content: <VisitReportsSection row={row} onChanged={changed} />,
    },
    {
      value: "prescrizione",
      label: "Prescrizione",
      content: (
        <Paper withBorder p="sm" radius="md">
          <DetailSection
            title="Prescrizione"
            actions={<PdfLink documentRef={row.documentRef} />}
          >
            <DetailField label="Data prescrizione">
              {formatDate(row.prescribedOn)}
            </DetailField>
            <DetailField label="NRE">
              <CopyButton value={row.nre} label="Copia NRE" />
            </DetailField>
          </DetailSection>
        </Paper>
      ),
    },
  ];

  return (
    <DetailDrawer
      title={row.title}
      badge={
        <StatusBadge tone={status.tone} manual={status.manual}>
          {status.label}
        </StatusBadge>
      }
      summary={
        at === undefined || unit === undefined ? (
          <Text size="sm">{`Prescritta il ${formatDate(row.prescribedOn)}`}</Text>
        ) : (
          <>
            <Text size="sm">
              {appointment?.relativeDay === undefined
                ? formatLongDateTime(at)
                : `${formatLongDateTime(at)} · ${appointment.relativeDay}`}
            </Text>
            <Text size="sm">{unit.displayName}</Text>
          </>
        )
      }
      actions={
        <IconAction
          label="Aggiorna"
          icon={RefreshCw}
          loading={data.refreshing}
          onClick={() => {
            void refresh();
          }}
        />
      }
      tabs={tabs}
      footer={
        updates.length === 0
          ? undefined
          : `Aggiornamenti — ${updates.join(" · ")}`
      }
      onClose={onClose}
    />
  );
}
